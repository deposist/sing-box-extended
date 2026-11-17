//go:build with_gvisor

package tailscale

import (
	"net/netip"
	"testing"

	nDNS "github.com/sagernet/tailscale/net/dns"
	"github.com/sagernet/tailscale/types/dnstype"
	"github.com/sagernet/tailscale/util/dnsname"

	"github.com/stretchr/testify/require"
)

func TestTailnetDomainsRoutePriority(t *testing.T) {
	domains := newTailnetDomains(&nDNS.Config{
		Routes: map[dnsname.FQDN][]*dnstype.Resolver{
			"ts.net.":              {{Addr: "192.0.2.1"}},
			"tailnet.ts.net.":      nil,
			"sub.tailnet.ts.net.":  {{Addr: "192.0.2.2"}},
			"example.org.":         {{Addr: "192.0.2.3"}},
			"private.example.org.": {{Addr: "192.0.2.4"}},
		},
	})
	for _, test := range []struct {
		domain    string
		suffix    string
		forwarded bool
	}{
		{"host.tailnet.ts.net.", "tailnet.ts.net.", false},
		{"host.sub.tailnet.ts.net.", "tailnet.ts.net.", false},
		{"host.private.example.org.", "private.example.org.", true},
		{"host.example.org.", "example.org.", true},
		{"host.notexample.org.", "", false},
	} {
		t.Run(test.domain, func(t *testing.T) {
			suffix, forwarded := domains.matchRoute(test.domain)
			require.Equal(t, test.suffix, suffix)
			require.Equal(t, test.forwarded, forwarded)
		})
	}
}

func TestTailnetDomainsPreferredDomain(t *testing.T) {
	transport := &DNSTransport{
		acceptSearchDomain: true,
		domains: newTailnetDomains(&nDNS.Config{
			Hosts: map[dnsname.FQDN][]netip.Addr{
				"Host.Tailnet.ts.net.": {netip.MustParseAddr("100.64.0.1")},
			},
			SearchDomains: []dnsname.FQDN{"Tailnet.ts.net."},
			Routes: map[dnsname.FQDN][]*dnstype.Resolver{
				"tailnet.ts.net.": nil,
			},
		}),
	}
	require.True(t, transport.PreferredDomain("HOST.TAILNET.TS.NET."))
	require.True(t, transport.PreferredDomain("host"))
	require.False(t, transport.PreferredDomain("unknown"))
	require.False(t, transport.PreferredDomain("unrelated.example.org."))
	transport.acceptSearchDomain = false
	require.False(t, transport.PreferredDomain("host"))
}
