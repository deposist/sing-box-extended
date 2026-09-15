package openvpn

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/json/badoption"

	"github.com/stretchr/testify/require"
)

// TestBuildClientTimingOptionsThreadsReconnectDelay proves the compatibility
// reconnect_delay option reaches the sing-openvpn supervisor timing, so a
// migrated legacy OpenVPN client keeps its fixed reconnect cadence. Missing
// wiring here would silently drop the value and fall back to exponential backoff.
func TestBuildClientTimingOptionsThreadsReconnectDelay(t *testing.T) {
	options := option.OpenVPNClientEndpointOptions{
		ReconnectDelay:  badoption.Duration(7 * time.Second),
		PingInterval:    badoption.Duration(10 * time.Second),
		HandshakeWindow: badoption.Duration(30 * time.Second),
	}
	timing := buildClientTimingOptions(options)

	require.Equal(t, 7*time.Second, timing.ReconnectDelay)
	require.Equal(t, 10*time.Second, timing.PingInterval)
	require.Equal(t, 30*time.Second, timing.HandWindow)
}

// TestClientEndpointOptionsDecodeReconnectDelayAndNameSuffix proves the JSON
// surface a migrated legacy config carries decodes into the new client option:
// the reconnect_delay timing field and the preserved server_name_type
// "name-suffix" TLS value both survive strict decode and reach the timing struct.
func TestClientEndpointOptionsDecodeReconnectDelayAndNameSuffix(t *testing.T) {
	raw := []byte(`{
		"mode": "tls",
		"reconnect_delay": "7s",
		"servers": [{"server": "10.0.0.1", "server_port": 1194}],
		"tls": {
			"server_name": "example.com",
			"server_name_type": "name-suffix"
		}
	}`)
	var options option.OpenVPNClientEndpointOptions
	require.NoError(t, json.Unmarshal(raw, &options))

	require.Equal(t, 7*time.Second, time.Duration(options.ReconnectDelay))
	require.NotNil(t, options.TLS)
	require.Equal(t, "name-suffix", options.TLS.ServerNameType)
	require.Equal(t, 7*time.Second, buildClientTimingOptions(options).ReconnectDelay)
}
