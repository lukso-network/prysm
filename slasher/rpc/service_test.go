package rpc

import (
	"context"
	"errors"
	"flag"
	"github.com/prysmaticlabs/prysm/cmd/slasher/flags"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	logTest "github.com/sirupsen/logrus/hooks/test"
	"github.com/urfave/cli/v2"
	"testing"
)

func TestLifecycle_OK(t *testing.T) {
	hook := logTest.NewGlobal()
	rpcService := NewService(context.Background(), &Config{
		Port:     "7348",
		CertFlag: "alice.crt",
		KeyFlag:  "alice.key",
	})

	rpcService.Start()

	require.LogsContain(t, hook, "listening on port")
	require.NoError(t, rpcService.Stop())
}

func TestStatus_CredentialError(t *testing.T) {
	credentialErr := errors.New("credentialError")
	s := &Service{credentialError: credentialErr}

	assert.ErrorContains(t, s.credentialError.Error(), s.Status())
}

func TestRPC_InsecureEndpoint(t *testing.T) {
	hook := logTest.NewGlobal()
	rpcService := NewService(context.Background(), &Config{
		Port: "7777",
	})

	rpcService.Start()

	require.LogsContain(t, hook, "listening on port")
	require.NoError(t, rpcService.Stop())
}

func TestRPC_AddressResolver(t *testing.T) {
	app := cli.App{}
	set := flag.NewFlagSet("test", 0)
	set.String(flags.RPCHost.Name, flags.RPCHost.Value, flags.RPCHost.Usage)
	set.Int(flags.RPCPort.Name, flags.RPCPort.Value, flags.RPCPort.Usage)
	ctx := cli.NewContext(&app, set, nil)

	tests := []struct {
		name string
		host string
		port string
	}{
		{
			name: "Test IPv4:Port dial",
			host: ctx.String(flags.RPCHost.Name),
		},
		{
			name: "Test IPC socket dial",
			host: "./slasher.ipc",
		},
		{
			name: "Test IPC socket dial (absolute unix path)",
			host: "/tmp/slasher.ipc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook := logTest.NewGlobal()
			rpcService := NewService(context.Background(), &Config{
				Host:     tt.host,
				Port:     ctx.String(flags.RPCPort.Name),
				CertFlag: "alice.crt",
				KeyFlag:  "alice.key",
			})

			rpcService.Start()

			require.LogsContain(t, hook, "listening on port")
			require.NoError(t, rpcService.Stop())

			assert.DeepEqual(t, tt.host, rpcService.cfg.Host)
			assert.DeepEqual(t, "4002", rpcService.cfg.Port)

			rpcService.Start()

			require.LogsContain(t, hook, "listening on port")
			assert.NoError(t, rpcService.Stop())
		})
	}
}
