package rpcutil_test

import (
	"fmt"
	"github.com/prysmaticlabs/prysm/shared/rpcutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"testing"
)

func TestResolveRpcAddressAndProtocol(t *testing.T) {
	tests := []struct {
		name           string
		bcHost         string
		bcPort         string
		bcHostProtocol string
		shouldPass     bool
		errMsg         string
	}{
		{
			name:           "Test IPv4:Port dial",
			bcHost:         "127.0.0.1:4000",
			bcHostProtocol: "tcp",
			shouldPass:     true,
		},
		{
			name:           "Test HTTP dial",
			bcHost:         "http://127.0.0.1:4000",
			bcHostProtocol: "tcp",
			shouldPass:     true,
		},
		{
			name:           "Test dial with separate port value",
			bcHost:         "127.0.0.1",
			bcPort:         "4000",
			bcHostProtocol: "tcp",
			shouldPass:     true,
		},
		{
			name:           "Test IPC socket dial",
			bcHost:         "./vanguard.ipc",
			bcHostProtocol: "unix",
			shouldPass:     true,
		},
		{
			name:           "Test IPC socket dial (absolute unix path)",
			bcHost:         "/tmp/vanguard.ipc",
			bcHostProtocol: "unix",
			shouldPass:     true,
		},
		{
			name:           "Test localhost:Port dial failure",
			bcHost:         "localhost:4000",
			bcHostProtocol: "",
			shouldPass:     false,
			errMsg:         "no known network transport layer for address",
		},
		{
			name:           "Test IPC socket dial failure",
			bcHost:         "./vanguard.abc",
			bcHostProtocol: "",
			shouldPass:     false,
			errMsg:         "invalid socket path",
		},
		{
			name:           "Test IPC socket dial failure",
			bcHost:         "./van",
			bcHostProtocol: "",
			shouldPass:     false,
			errMsg:         "invalid socket path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			address, protocol, err := rpcutil.ResolveRpcAddressAndProtocol(tt.bcHost, tt.bcPort)
			if !tt.shouldPass {
				require.ErrorContains(t, fmt.Sprintf("%s %q", tt.errMsg, tt.bcHost), err)
			} else {
				require.NoError(t, err)
			}
			if tt.bcPort != "" {
				require.DeepEqual(t, fmt.Sprintf("%s:%s", tt.bcHost, tt.bcPort), address)
			} else {
				require.DeepEqual(t, tt.bcHost, address)
			}
			require.DeepEqual(t, tt.bcHostProtocol, protocol)
		})
	}
}
