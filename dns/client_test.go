package dns

import (
	"context"
	"net/netip"
	"testing"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/interrupt"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"

	mDNS "github.com/miekg/dns"
	"github.com/stretchr/testify/require"
)

func clientTestMessage() *mDNS.Msg {
	message := new(mDNS.Msg)
	message.SetQuestion("client.example.org.", mDNS.TypeA)
	return message
}

func TestClientDNSConnectionIsInternal(t *testing.T) {
	client := NewClient(ClientOptions{Context: context.Background(), DisableCache: true})
	transport := &fakeDNSTransport{tag: "resolver"}
	ctx := interrupt.ContextWithIsExternalConnection(context.Background(), true)
	operation, _, status, err := client.beginExchange(ctx, transport, clientTestMessage(), adapter.DNSQueryOptions{}, nil, true)
	require.NoError(t, err)
	require.Equal(t, exchangeReady, status)
	defer operation.release()
	require.False(t, interrupt.IsExternalConnectionFromContext(operation.ctx))
	require.True(t, interrupt.IsExternalConnectionFromContext(ctx))
	tag, loaded := adapter.DNSTransportTagFromContext(operation.ctx)
	require.True(t, loaded)
	require.Equal(t, "resolver", tag)
	require.Equal(t, []string{"resolver"}, transportStackFromContext(operation.ctx))
}

func TestClientRejectsDNSLoops(t *testing.T) {
	for _, test := range []struct {
		name string
		ctx  context.Context
		err  string
	}{
		{
			name: "transport loopback",
			ctx:  adapter.ContextWithDNSTransportTag(context.Background(), "resolver"),
			err:  "DNS query loopback in transport[resolver]",
		},
		{
			name: "indirect transport loop",
			ctx:  contextWithTransportStack(context.Background(), []string{"resolver", "bootstrap"}),
			err:  "DNS resolution loop detected: resolver -> bootstrap -> resolver",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := NewClient(ClientOptions{Context: context.Background()})
			transport := &fakeDNSTransport{tag: "resolver"}
			message := clientTestMessage()
			operation, response, status, err := client.beginExchange(test.ctx, transport, message, adapter.DNSQueryOptions{}, nil, true)
			require.ErrorContains(t, err, test.err)
			require.Nil(t, operation)
			require.Nil(t, response)
			require.Equal(t, exchangeDone, status)
			operation, _, status, err = client.beginExchange(context.Background(), transport, message, adapter.DNSQueryOptions{}, nil, false)
			require.NoError(t, err)
			require.Equal(t, exchangeReady, status)
			operation.release()
		})
	}
}

func TestClientCachedResponseChecker(t *testing.T) {
	for _, lookup := range []bool{false, true} {
		name := "exchange"
		if lookup {
			name = "lookup"
		}
		t.Run(name, func(t *testing.T) {
			client := NewClient(ClientOptions{Context: context.Background(), Logger: log.NewNOPFactory().Logger()})
			transport := &fakeDNSTransport{tag: "resolver", address: netip.MustParseAddr("192.0.2.1")}
			message := clientTestMessage()
			_, err := client.Exchange(context.Background(), transport, message, adapter.DNSQueryOptions{}, nil)
			require.NoError(t, err)
			checkerCalls := 0
			checker := func(response *mDNS.Msg) bool {
				checkerCalls++
				return false
			}
			if lookup {
				addresses, err := client.Lookup(context.Background(), transport, "client.example.org", adapter.DNSQueryOptions{Strategy: C.DomainStrategyIPv4Only}, checker)
				require.ErrorIs(t, err, ErrResponseRejectedCached)
				require.Empty(t, addresses)
			} else {
				response, err := client.Exchange(context.Background(), transport, message, adapter.DNSQueryOptions{}, checker)
				require.ErrorIs(t, err, ErrResponseRejectedCached)
				require.Nil(t, response)
			}
			require.Equal(t, 1, checkerCalls)
			require.EqualValues(t, 1, transport.queryCount.Load())
			response, err := client.Exchange(context.Background(), transport, message, adapter.DNSQueryOptions{}, nil)
			require.NoError(t, err)
			require.NotNil(t, response)
			require.EqualValues(t, 1, transport.queryCount.Load())
		})
	}
}
