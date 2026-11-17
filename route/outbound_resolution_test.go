package route

import (
	"context"
	"io"
	"net"
	"strings"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/adapter/outbound"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type resolutionGroup struct {
	outbound.Adapter
	next  adapter.Outbound
	calls int
}

func (g *resolutionGroup) All() []string                     { return nil }
func (g *resolutionGroup) AttachConnection(io.Closer) func() { return func() {} }
func (g *resolutionGroup) DialContext(context.Context, string, M.Socksaddr) (net.Conn, error) {
	panic("resolution must not dial")
}

func (g *resolutionGroup) ListenPacket(context.Context, M.Socksaddr) (net.PacketConn, error) {
	panic("resolution must not listen")
}

func (g *resolutionGroup) Selected(string) adapter.Outbound {
	g.calls++
	if g.calls > 1 {
		return nil
	}
	return g.next
}

func TestResolveOutboundPreservesSelfRoutingFallback(t *testing.T) {
	for _, network := range []string{N.NetworkTCP, N.NetworkUDP} {
		t.Run(network, func(t *testing.T) {
			group := &resolutionGroup{Adapter: outbound.NewAdapter("fallback", "fallback", []string{network}, nil)}
			group.next = group
			chain, err := resolveOutbound(group, network)
			if err != nil {
				t.Fatalf("self-routing fallback must retain its retry policy: %v", err)
			}
			if len(chain) != 1 || chain[0] != group || group.calls != 1 {
				t.Fatalf("fallback resolved away or repeatedly selected: chain=%v calls=%d", chain, group.calls)
			}
		})
	}
}

func TestResolveOutboundRejectsSelectedGroupCycle(t *testing.T) {
	first := &resolutionGroup{Adapter: outbound.NewAdapter("selector", "first", []string{N.NetworkTCP}, nil)}
	second := &resolutionGroup{Adapter: outbound.NewAdapter("selector", "second", []string{N.NetworkTCP}, nil)}
	first.next, second.next = second, first
	_, err := resolveOutbound(first, N.NetworkTCP)
	if err == nil || !strings.Contains(err.Error(), "loop") {
		t.Fatalf("selected group cycle must return a routing loop error: %v", err)
	}
}
