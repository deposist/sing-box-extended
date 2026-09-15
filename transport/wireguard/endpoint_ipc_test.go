package wireguard

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEndpointIPCBeforeStart(t *testing.T) {
	endpoint := new(Endpoint)

	_, err := endpoint.IpcGet()
	require.EqualError(t, err, "wireguard device is not started")
	require.EqualError(t, endpoint.IpcSet(""), "wireguard device is not started")
}
