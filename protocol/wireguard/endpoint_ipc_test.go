package wireguard

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEndpointIPCBeforeInitialization(t *testing.T) {
	endpoint := new(Endpoint)

	_, err := endpoint.IpcGet()
	require.EqualError(t, err, "wireguard endpoint is not initialized")
	require.EqualError(t, endpoint.IpcSet(""), "wireguard endpoint is not initialized")
}
