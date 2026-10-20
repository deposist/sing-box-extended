package service

import (
	"context"
	"os"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
)

var _ adapter.ServiceManager = (*Manager)(nil)

type Manager struct {
	registry     adapter.ServiceRegistry
	access       sync.Mutex
	scope        *adapter.Scope
	services     []adapter.Service
	serviceByTag map[string]adapter.Service
}

func NewManager(registry adapter.ServiceRegistry) *Manager {
	return &Manager{
		registry:     registry,
		serviceByTag: make(map[string]adapter.Service),
	}
}

func (m *Manager) Start(stage adapter.StartStage, scope *adapter.Scope) error {
	m.access.Lock()
	if stage == adapter.StartStateInitialize {
		m.scope = scope
	}
	services := m.services
	m.access.Unlock()
	for _, service := range services {
		name := "service/" + service.Type() + "[" + service.Tag() + "]"
		err := scope.Start(name, service, stage)
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) Services() []adapter.Service {
	m.access.Lock()
	defer m.access.Unlock()
	return m.services
}

func (m *Manager) Get(tag string) (adapter.Service, bool) {
	m.access.Lock()
	service, found := m.serviceByTag[tag]
	m.access.Unlock()
	return service, found
}

func (m *Manager) Remove(tag string) error {
	m.access.Lock()
	service, found := m.serviceByTag[tag]
	if !found {
		m.access.Unlock()
		return os.ErrInvalid
	}
	delete(m.serviceByTag, tag)
	m.services = common.Filter(m.services, func(it adapter.Service) bool {
		return it != service
	})
	scope := m.scope
	m.access.Unlock()
	if scope == nil {
		return nil
	}
	return scope.Remove(service)
}

func (m *Manager) Create(ctx context.Context, logger log.ContextLogger, tag string, serviceType string, options any) error {
	service, err := m.registry.Create(ctx, logger, tag, serviceType, options)
	if err != nil {
		return err
	}
	m.access.Lock()
	defer m.access.Unlock()
	_, loaded := m.serviceByTag[tag]
	if loaded {
		return E.New("duplicate service tag: ", tag)
	}
	m.services = append(m.services, service)
	m.serviceByTag[tag] = service
	return nil
}
