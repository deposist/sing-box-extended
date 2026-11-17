package group

import (
	"context"
	"errors"
	"net"
	"reflect"
	"regexp"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	"github.com/sagernet/sing-box/common/interrupt"
	"github.com/sagernet/sing-box/common/urltest"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type testOutbound struct {
	outbound.Adapter
	dial   func() (net.Conn, error)
	listen func() (net.PacketConn, error)
}

func (o *testOutbound) DialContext(context.Context, string, M.Socksaddr) (net.Conn, error) {
	if o.dial != nil {
		return o.dial()
	}
	return nil, errors.New("unavailable")
}

func (o *testOutbound) ListenPacket(context.Context, M.Socksaddr) (net.PacketConn, error) {
	if o.listen != nil {
		return o.listen()
	}
	return nil, errors.New("unavailable")
}

func newTestOutbound(tag string) *testOutbound {
	return &testOutbound{Adapter: outbound.NewAdapter(C.TypeDirect, tag, []string{N.NetworkTCP, N.NetworkUDP}, nil)}
}

func TestRealTagNestedNetworkSelection(t *testing.T) {
	tcp := newTestOutbound("tcp")
	udp := newTestOutbound("udp")
	urlTest := &URLTest{
		Adapter: outbound.NewAdapter(C.TypeURLTest, "urltest", []string{N.NetworkTCP, N.NetworkUDP}, nil),
		group:   &URLTestGroup{selectedOutboundTCP: tcp, selectedOutboundUDP: udp},
	}
	selector := &Selector{Adapter: outbound.NewAdapter(C.TypeSelector, "selector", nil, nil)}
	selector.selected.Store(urlTest)
	for network, expected := range map[string]string{N.NetworkTCP: tcp.Tag(), N.NetworkUDP: udp.Tag()} {
		if actual := RealTag(selector, network); actual != expected {
			t.Errorf("%s tag = %q, want %q", network, actual, expected)
		}
	}
	selector.selected.Store(nil)
	if actual := RealTag(selector, N.NetworkTCP); actual != "" {
		t.Fatalf("empty selection tag = %q", actual)
	}
}

func TestRealTagSelectionCycle(t *testing.T) {
	first := &Selector{Adapter: outbound.NewAdapter(C.TypeSelector, "first", nil, nil)}
	second := &Selector{Adapter: outbound.NewAdapter(C.TypeSelector, "second", nil, nil)}
	first.selected.Store(second)
	second.selected.Store(first)
	if actual := RealTag(first, N.NetworkTCP); actual != "" {
		t.Fatalf("cyclic selection tag = %q", actual)
	}
}

type testPacketConn struct {
	net.PacketConn
}

func (c *testPacketConn) Close() error { return nil }

func TestFallbackSelectionPreservesRetries(t *testing.T) {
	for _, network := range []string{N.NetworkTCP, N.NetworkUDP} {
		t.Run(network, func(t *testing.T) {
			first := newTestOutbound("first")
			second := newTestOutbound("second")
			var firstCalls, secondCalls int
			first.dial = func() (net.Conn, error) {
				firstCalls++
				return nil, errors.New("unavailable")
			}
			second.dial = func() (net.Conn, error) {
				secondCalls++
				conn, peer := net.Pipe()
				t.Cleanup(func() { peer.Close() })
				return conn, nil
			}
			first.listen = func() (net.PacketConn, error) {
				firstCalls++
				return nil, errors.New("unavailable")
			}
			second.listen = func() (net.PacketConn, error) {
				secondCalls++
				return &testPacketConn{}, nil
			}
			fallback := &Fallback{
				Adapter:          outbound.NewAdapter(C.TypeFallback, "fallback", []string{N.NetworkTCP, N.NetworkUDP}, nil),
				logger:           log.NewNOPFactory().Logger(),
				tags:             []string{first.Tag(), second.Tag()},
				outbounds:        map[string]adapter.Outbound{first.Tag(): first, second.Tag(): second},
				lastUsedOutbound: first.Tag(),
				blacklistTimeout: time.Minute,
				blacklist:        make(map[string]time.Time),
			}
			selected := fallback.Selected(network)
			if selected != fallback {
				t.Fatal("fallback resolved to a leaf and lost retry policy")
			}
			for range 2 {
				if network == N.NetworkTCP {
					conn, err := selected.DialContext(context.Background(), network, M.Socksaddr{})
					if err != nil {
						t.Fatal(err)
					}
					conn.Close()
				} else {
					conn, err := selected.ListenPacket(context.Background(), M.Socksaddr{})
					if err != nil {
						t.Fatal(err)
					}
					conn.Close()
				}
			}
			if firstCalls != 1 || secondCalls != 2 {
				t.Fatalf("attempts = (%d, %d), want (1, 2)", firstCalls, secondCalls)
			}
			if actual := RealTag(fallback, network); actual != second.Tag() {
				t.Fatalf("fallback history tag = %q, want %q", actual, second.Tag())
			}
		})
	}
}

type testProvider struct {
	adapter.Provider
	outbounds []adapter.Outbound
}

func (p *testProvider) Outbounds() []adapter.Outbound { return p.outbounds }

func TestSelectorProviderRefreshPreservesFilters(t *testing.T) {
	first := newTestOutbound("US-first")
	second := newTestOutbound("US-second")
	provider := &testProvider{outbounds: []adapter.Outbound{first, newTestOutbound("RU-first"), newTestOutbound("US-excluded")}}
	selector := &Selector{
		Adapter:        outbound.NewAdapter(C.TypeSelector, "selector", nil, nil),
		ctx:            context.Background(),
		providers:      map[string]adapter.Provider{"provider": provider},
		providerTags:   []string{"provider"},
		outboundsCache: make(map[string][]adapter.Outbound),
		outbounds:      make(map[string]adapter.Outbound),
		include:        regexp.MustCompile("^US-"),
		exclude:        regexp.MustCompile("excluded$"),
		interruptGroup: interrupt.NewGroup(),
	}
	if err := selector.onProviderUpdated("provider"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(selector.All(), []string{first.Tag()}) || selector.Selected(N.NetworkTCP) != first {
		t.Fatalf("filtered selection = %v / %v", selector.All(), selector.Now())
	}
	provider.outbounds = []adapter.Outbound{second}
	if err := selector.onProviderUpdated("provider"); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(selector.All(), []string{second.Tag()}) || selector.Selected(N.NetworkUDP) != second {
		t.Fatalf("refreshed selection = %v / %v", selector.All(), selector.Now())
	}
	if selector.SelectOutbound(first.Tag()) {
		t.Fatal("removed provider outbound remained selectable")
	}
}

func TestCanceledURLTestPreservesSelection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	oldOutbound := newTestOutbound("old")
	newOutbound := newTestOutbound("new")
	group := &URLTestGroup{
		outbounds:           []adapter.Outbound{newOutbound},
		history:             urltest.NewHistoryStorage(),
		logger:              log.NewNOPFactory().Logger(),
		selectedOutboundTCP: oldOutbound,
		selectedOutboundUDP: oldOutbound,
		interruptGroup:      interrupt.NewGroup(),
	}
	if _, err := group.URLTest(ctx); err != nil {
		t.Fatal(err)
	}
	if group.selectedOutboundTCP != oldOutbound || group.selectedOutboundUDP != oldOutbound {
		t.Fatal("canceled URL test changed the active outbound")
	}
}
