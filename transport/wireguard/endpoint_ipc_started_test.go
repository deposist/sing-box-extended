//go:build with_gvisor

package wireguard

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"net/netip"
	"strings"
	"testing"

	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"

	"github.com/stretchr/testify/require"
)

// newTestKey returns a fresh X25519 keypair: base64 private (for the endpoint)
// and base64 public (for a peer). The transport encodes the base64 public key
// to hex internally, so base64 is the wire format a panel config carries.
func newTestKey(t *testing.T) (privB64 string, pubB64 string, pubHex string) {
	t.Helper()
	curve := ecdh.X25519()
	priv, err := curve.GenerateKey(rand.Reader)
	require.NoError(t, err)
	privB64 = base64.StdEncoding.EncodeToString(priv.Bytes())
	pubB64 = base64.StdEncoding.EncodeToString(priv.PublicKey().Bytes())
	pubHex = hex.EncodeToString(priv.PublicKey().Bytes())
	return
}

// TestEndpointIPCOnStartedDevice exercises the live UAPI on a real userspace
// WireGuard endpoint: IpcGet reflects configured peers, IpcSet adds and removes
// a peer without recreating the endpoint, and after Close the guard error is
// returned again. This is the path the panel's WithWireGuardIPC drives.
func TestEndpointIPCOnStartedDevice(t *testing.T) {
	ctx := context.Background()
	_, peer1PubB64, peer1PubHex := newTestKey(t)
	_, _, peer2PubHex := newTestKey(t)
	privB64, _, _ := newTestKey(t)

	d, err := dialer.NewDefault(ctx, option.DialerOptions{})
	require.NoError(t, err)

	endpoint, err := NewEndpoint(EndpointOptions{
		Context:    ctx,
		Logger:     log.NewNOPFactory().Logger(),
		System:     false,
		Dialer:     d,
		MTU:        1408,
		Address:    []netip.Prefix{netip.MustParsePrefix("10.0.0.1/24")},
		PrivateKey: privB64,
		ListenPort: 0, // no OS socket bind
		Peers: []PeerOptions{
			{
				PublicKey:  peer1PubB64,
				AllowedIPs: []netip.Prefix{netip.MustParsePrefix("10.0.1.0/24")},
			},
		},
	})
	require.NoError(t, err)

	require.NoError(t, endpoint.Start(false))
	defer endpoint.Close()

	// IpcGet reports the initially configured peer.
	got, err := endpoint.IpcGet()
	require.NoError(t, err)
	require.Contains(t, got, "public_key="+peer1PubHex)
	require.NotContains(t, got, peer2PubHex)

	// IpcSet adds a second peer without recreating the endpoint.
	require.NoError(t, endpoint.IpcSet("public_key="+peer2PubHex+"\nallowed_ip=10.0.2.0/24\n"))
	got, err = endpoint.IpcGet()
	require.NoError(t, err)
	require.Contains(t, got, "public_key="+peer1PubHex)
	require.Contains(t, got, "public_key="+peer2PubHex)

	// IpcSet removes every peer.
	require.NoError(t, endpoint.IpcSet("replace_peers=true\n"))
	got, err = endpoint.IpcGet()
	require.NoError(t, err)
	require.NotContains(t, got, "public_key="+peer1PubHex)
	require.NotContains(t, got, "public_key="+peer2PubHex)
	require.False(t, strings.Contains(got, "public_key="), "no peers should remain, got: %q", got)
}

// TestEndpointIPCAfterCloseReturnsGuardError proves the accessors degrade to the
// documented guard error once the device is torn down, matching the panel's
// dynamic interface assertion on a stopped endpoint.
func TestEndpointIPCAfterCloseReturnsGuardError(t *testing.T) {
	ctx := context.Background()
	privB64, _, _ := newTestKey(t)
	_, peerPubB64, _ := newTestKey(t)

	d, err := dialer.NewDefault(ctx, option.DialerOptions{})
	require.NoError(t, err)

	endpoint, err := NewEndpoint(EndpointOptions{
		Context:    ctx,
		Logger:     log.NewNOPFactory().Logger(),
		System:     false,
		Dialer:     d,
		MTU:        1408,
		Address:    []netip.Prefix{netip.MustParsePrefix("10.0.0.1/24")},
		PrivateKey: privB64,
		ListenPort: 0,
		Peers: []PeerOptions{
			{
				PublicKey:  peerPubB64,
				AllowedIPs: []netip.Prefix{netip.MustParsePrefix("10.0.1.0/24")},
			},
		},
	})
	require.NoError(t, err)
	require.NoError(t, endpoint.Start(false))

	_, err = endpoint.IpcGet()
	require.NoError(t, err)

	require.NoError(t, endpoint.Close())

	_, err = endpoint.IpcGet()
	require.EqualError(t, err, "wireguard device is not started")
	require.EqualError(t, endpoint.IpcSet(""), "wireguard device is not started")
}
