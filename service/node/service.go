package node

import (
	"context"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	boxService "github.com/sagernet/sing-box/adapter/service"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	CM "github.com/sagernet/sing-box/service/manager/constant"
	"github.com/sagernet/sing-box/service/node/constant"
	"github.com/sagernet/sing-box/service/node/inbound"
	"github.com/sagernet/sing-box/service/node/limiter"
	pb "github.com/sagernet/sing-box/service/node_manager_api/manager"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/service"
)

func RegisterService(registry *boxService.Registry) {
	boxService.Register[option.NodeServiceOptions](registry, C.TypeNode, NewService)
}

type Service struct {
	boxService.Adapter
	ctx               context.Context
	cancel            context.CancelFunc
	logger            log.ContextLogger
	inboundManagers   map[string]constant.InboundManager
	bandwidthManager  constant.BandwidthLimiterManager
	connectionManager constant.ConnectionLimiterManager
	trafficManager    constant.TrafficLimiterManager
	rateManager       constant.RateLimiterManager
	options           option.NodeServiceOptions

	nodeManager CM.NodeManager

	helloProvider func() *pb.NodeHello

	mtx sync.Mutex
}

func NewService(ctx context.Context, logger log.ContextLogger, tag string, options option.NodeServiceOptions) (adapter.Service, error) {
	ctx, cancel := context.WithCancel(ctx)
	return &Service{
		Adapter: boxService.NewAdapter(C.TypeNode, tag),
		ctx:     ctx,
		cancel:  cancel,
		logger:  logger,
		options: options,
	}, nil
}

func (s *Service) Start(stage adapter.StartStage, scope *adapter.Scope) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	scope.Add(s.Close)
	boxManager := service.FromContext[adapter.ServiceManager](s.ctx)
	serviceManager, ok := boxManager.Get(s.options.Manager)
	if !ok {
		return E.New("manager ", s.options.Manager, " not found")
	}
	nodeManager, ok := serviceManager.(CM.NodeManager)
	if !ok {
		return E.New("invalid ", s.options.Manager, " manager")
	}
	s.nodeManager = nodeManager
	inboundManager := service.FromContext[adapter.InboundManager](s.ctx)
	outboundManager := service.FromContext[adapter.OutboundManager](s.ctx)
	s.inboundManagers = map[string]constant.InboundManager{
		"anytls":      inbound.NewAnyTLSManager(),
		"http":        inbound.NewHTTPManager(),
		"hysteria":    inbound.NewHysteriaManager(),
		"hysteria2":   inbound.NewHysteria2Manager(),
		"mixed":       inbound.NewMixedManager(),
		"mtproxy":     inbound.NewMTProxyManager(),
		"naive":       inbound.NewNaiveManager(),
		"socks":       inbound.NewSocksManager(),
		"trojan":      inbound.NewTrojanManager(),
		"trusttunnel": inbound.NewTrustTunnelManager(),
		"tuic":        inbound.NewTUICManager(),
		"vless":       inbound.NewVLESSManager(),
		"vmess":       inbound.NewVMessManager(),
		"ssh":         inbound.NewSSHManager(),
	}
	s.connectionManager = limiter.NewConnectionLimiterManager(s.ctx, nodeManager, s.logger)
	s.bandwidthManager = limiter.NewBandwidthLimiterManager(s.ctx, nodeManager, s.logger)
	s.trafficManager = limiter.NewTrafficLimiterManager(s.ctx, nodeManager, s.logger)
	s.rateManager = limiter.NewRateLimiterManager(s.ctx, nodeManager, s.logger)
	for _, tag := range s.options.Inbounds {
		inbound, ok := inboundManager.Get(tag)
		if !ok {
			return E.New("inbound ", tag, " not found")
		}
		inboundManager, ok := s.inboundManagers[inbound.Type()]
		if !ok {
			return E.New("inbound manager for ", tag, " not found")
		}
		err := inboundManager.AddUserManager(inbound)
		if err != nil {
			return err
		}
	}
	for _, limiter := range s.options.ConnectionLimiters {
		outbound, ok := outboundManager.Outbound(limiter)
		if !ok {
			return E.New("outbound ", limiter, " not found")
		}
		err := s.connectionManager.AddConnectionLimiterStrategyManager(outbound)
		if err != nil {
			return err
		}
	}
	for _, limiter := range s.options.BandwidthLimiters {
		outbound, ok := outboundManager.Outbound(limiter)
		if !ok {
			return E.New("outbound " + limiter + " not found")
		}
		err := s.bandwidthManager.AddBandwidthLimiterStrategyManager(outbound)
		if err != nil {
			return err
		}
	}
	for _, limiter := range s.options.TrafficLimiters {
		outbound, ok := outboundManager.Outbound(limiter)
		if !ok {
			return E.New("outbound ", limiter, " not found")
		}
		err := s.trafficManager.AddTrafficLimiterStrategyManager(outbound)
		if err != nil {
			return err
		}
	}
	for _, limiter := range s.options.RateLimiters {
		outbound, ok := outboundManager.Outbound(limiter)
		if !ok {
			return E.New("outbound ", limiter, " not found")
		}
		err := s.rateManager.AddRateLimiterStrategyManager(outbound)
		if err != nil {
			return err
		}
	}
	if announcer, ok := nodeManager.(interface{ SetHello(*pb.NodeHello) }); ok {
		announcer.SetHello(s.Hello())
	}
	return nodeManager.AddNode(s.options.UUID, s)
}

// NodeHelloProtocolVersion is the self-description format this build speaks.
// A manager that does not understand the newest version can still fall back to
// the plain uuid handshake, because hello is additive.
const NodeHelloProtocolVersion = 1

// SetHelloProvider installs a function that produces the node self-description
// instead of the built-in one. An embedding application (a panel) knows the link
// format of its inbounds and can therefore fill each inbound's opaque
// descriptor, which the core itself cannot. The provider runs on every connect.
func (s *Service) SetHelloProvider(provider func() *pb.NodeHello) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	s.helloProvider = provider
}

// Hello returns the self-description this node announces: the provider's when
// one is installed, the built-in inventory otherwise.
func (s *Service) Hello() *pb.NodeHello {
	s.mtx.Lock()
	provider := s.helloProvider
	s.mtx.Unlock()
	if provider != nil {
		return provider()
	}
	return s.buildHello()
}

func (s *Service) buildHello() *pb.NodeHello {
	hello := &pb.NodeHello{
		ProtocolVersion: NodeHelloProtocolVersion,
		CoreVersion:     C.Version,
	}
	inboundTypes := make(map[string]string, len(s.options.Inbounds))
	for type_, manager := range s.inboundManagers {
		for _, tag := range manager.GetUserManagerTags() {
			inboundTypes[tag] = type_
		}
	}
	for _, tag := range s.options.Inbounds {
		hello.Inbounds = append(hello.Inbounds, &pb.NodeInbound{Tag: tag, Type: inboundTypes[tag]})
	}
	appendLimiters := func(variant string, tags []string) {
		for _, tag := range tags {
			hello.Limiters = append(hello.Limiters, &pb.NodeLimiter{Tag: tag, Type: variant, Variant: variant})
		}
	}
	appendLimiters(C.TypeBandwidthLimiter, s.bandwidthManager.GetBandwidthLimiterStrategyManagerTags())
	appendLimiters(C.TypeConnectionLimiter, s.connectionManager.GetConnectionLimiterStrategyManagerTags())
	appendLimiters(C.TypeTrafficLimiter, s.trafficManager.GetTrafficLimiterStrategyManagerTags())
	appendLimiters(C.TypeRateLimiter, s.rateManager.GetRateLimiterStrategyManagerTags())
	return hello
}

func (s *Service) UpdateUser(user CM.User) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	manager, ok := s.inboundManagers[user.Type]
	if !ok {
		return
	}
	userManager, ok := manager.GetUserManager(user.Inbound)
	if !ok {
		return
	}
	userManager.UpdateUser(user)
}

// UpdateUsers applies a full snapshot. Users are grouped by their inbound tag,
// not by protocol type: an inbound must only ever see the users assigned to it.
// Grouping by type handed every inbound of the same protocol the whole list,
// which leaked users into inbounds they were never assigned to.
//
// Per-inbound UpdateUsers replaces the manager's set, so a user absent from the
// snapshot is dropped - including one deleted while this node was offline.
func (s *Service) UpdateUsers(users []CM.User) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	byInbound := make(map[string][]CM.User)
	for _, user := range users {
		byInbound[user.Inbound] = append(byInbound[user.Inbound], user)
	}
	seen := make(map[string]struct{}, len(byInbound))
	for inboundTag, inboundUsers := range byInbound {
		seen[inboundTag] = struct{}{}
		manager, ok := s.inboundManagers[inboundUsers[0].Type]
		if !ok {
			continue
		}
		userManager, ok := manager.GetUserManager(inboundTag)
		if !ok {
			continue
		}
		userManager.UpdateUsers(inboundUsers)
	}
	// An inbound that received no users in this snapshot is empty.
	for _, manager := range s.inboundManagers {
		for _, tag := range manager.GetUserManagerTags() {
			if _, ok := seen[tag]; ok {
				continue
			}
			userManager, ok := manager.GetUserManager(tag)
			if !ok {
				continue
			}
			userManager.UpdateUsers(nil)
		}
	}
}

func (s *Service) DeleteUser(user CM.User) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	manager, ok := s.inboundManagers[user.Type]
	if !ok {
		return
	}
	userManager, ok := manager.GetUserManager(user.Inbound)
	if !ok {
		return
	}
	userManager.DeleteUser(user.Username)
}

func (s *Service) UpdateConnectionLimiter(limiter CM.ConnectionLimiter) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	manager, ok := s.connectionManager.GetConnectionLimiterStrategyManager(limiter.Outbound)
	if !ok {
		return
	}
	manager.UpdateConnectionLimiter(limiter)
}

func (s *Service) UpdateConnectionLimiters(limiters []CM.ConnectionLimiter) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	for _, limiter := range limiters {
		manager, ok := s.connectionManager.GetConnectionLimiterStrategyManager(limiter.Outbound)
		if !ok {
			continue
		}
		manager.UpdateConnectionLimiters(limiters)
	}
}

func (s *Service) DeleteConnectionLimiter(limiter CM.ConnectionLimiter) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	manager, ok := s.connectionManager.GetConnectionLimiterStrategyManager(limiter.Outbound)
	if !ok {
		return
	}
	manager.DeleteConnectionLimiter(limiter.Username)
}

func (s *Service) UpdateBandwidthLimiter(limiter CM.BandwidthLimiter) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	manager, ok := s.bandwidthManager.GetBandwidthLimiterStrategyManager(limiter.Outbound)
	if !ok {
		return
	}
	manager.UpdateBandwidthLimiter(limiter)
}

func (s *Service) UpdateBandwidthLimiters(limiters []CM.BandwidthLimiter) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	for _, limiter := range limiters {
		manager, ok := s.bandwidthManager.GetBandwidthLimiterStrategyManager(limiter.Outbound)
		if !ok {
			continue
		}
		manager.UpdateBandwidthLimiters(limiters)
	}
}

func (s *Service) DeleteBandwidthLimiter(limiter CM.BandwidthLimiter) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	manager, ok := s.bandwidthManager.GetBandwidthLimiterStrategyManager(limiter.Outbound)
	if !ok {
		return
	}
	manager.DeleteBandwidthLimiter(limiter.Username)
}

func (s *Service) UpdateTrafficLimiter(limiter CM.TrafficLimiter) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	manager, ok := s.trafficManager.GetTrafficLimiterStrategyManager(limiter.Outbound)
	if !ok {
		return
	}
	manager.UpdateTrafficLimiter(limiter)
}

func (s *Service) UpdateTrafficLimiters(limiters []CM.TrafficLimiter) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	for _, limiter := range limiters {
		manager, ok := s.trafficManager.GetTrafficLimiterStrategyManager(limiter.Outbound)
		if !ok {
			continue
		}
		manager.UpdateTrafficLimiters(limiters)
	}
}

func (s *Service) DeleteTrafficLimiter(limiter CM.TrafficLimiter) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	manager, ok := s.trafficManager.GetTrafficLimiterStrategyManager(limiter.Outbound)
	if !ok {
		return
	}
	manager.DeleteTrafficLimiter(limiter.Username)
}

func (s *Service) UpdateRateLimiter(limiter CM.RateLimiter) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	manager, ok := s.rateManager.GetRateLimiterStrategyManager(limiter.Outbound)
	if !ok {
		return
	}
	manager.UpdateRateLimiter(limiter)
}

func (s *Service) UpdateRateLimiters(limiters []CM.RateLimiter) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	for _, limiter := range limiters {
		manager, ok := s.rateManager.GetRateLimiterStrategyManager(limiter.Outbound)
		if !ok {
			continue
		}
		manager.UpdateRateLimiters(limiters)
	}
}

func (s *Service) DeleteRateLimiter(limiter CM.RateLimiter) {
	s.mtx.Lock()
	defer s.mtx.Unlock()
	manager, ok := s.rateManager.GetRateLimiterStrategyManager(limiter.Outbound)
	if !ok {
		return
	}
	manager.DeleteRateLimiter(limiter.Username)
}

func (s *Service) IsLocal() bool {
	return true
}

func (s *Service) IsOnline() bool {
	return true
}

func (s *Service) Close() error {
	s.cancel()
	return nil
}
