package rpc

import (
	"context"
	"errors"
	"flag"
	"github.com/prysmaticlabs/prysm/cmd/beacon-chain/flags"
	"github.com/urfave/cli/v2"
	"io/ioutil"
	"testing"
	"time"

	mock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	mockPOW "github.com/prysmaticlabs/prysm/beacon-chain/powchain/testing"
	mockSync "github.com/prysmaticlabs/prysm/beacon-chain/sync/initial-sync/testing"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"github.com/sirupsen/logrus"
	logTest "github.com/sirupsen/logrus/hooks/test"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetOutput(ioutil.Discard)
}

func TestLifecycle_OK(t *testing.T) {
	hook := logTest.NewGlobal()
	chainService := &mock.ChainService{
		Genesis: time.Now(),
	}
	rpcService := NewService(context.Background(), &Config{
		Port:                "7348",
		SyncService:         &mockSync.Sync{IsSyncing: false},
		BlockReceiver:       chainService,
		AttestationReceiver: chainService,
		HeadFetcher:         chainService,
		GenesisTimeFetcher:  chainService,
		POWChainService:     &mockPOW.POWChain{},
		StateNotifier:       chainService.StateNotifier(),
	})

	rpcService.Start()

	require.LogsContain(t, hook, "listening on port")
	assert.NoError(t, rpcService.Stop())
}

func TestStatus_CredentialError(t *testing.T) {
	credentialErr := errors.New("credentialError")
	s := &Service{
		cfg:             &Config{SyncService: &mockSync.Sync{IsSyncing: false}},
		credentialError: credentialErr,
	}

	assert.ErrorContains(t, s.credentialError.Error(), s.Status())
}

func TestRPC_InsecureEndpoint(t *testing.T) {
	hook := logTest.NewGlobal()
	chainService := &mock.ChainService{Genesis: time.Now()}
	rpcService := NewService(context.Background(), &Config{
		Port:                "7777",
		SyncService:         &mockSync.Sync{IsSyncing: false},
		BlockReceiver:       chainService,
		GenesisTimeFetcher:  chainService,
		AttestationReceiver: chainService,
		HeadFetcher:         chainService,
		POWChainService:     &mockPOW.POWChain{},
		StateNotifier:       chainService.StateNotifier(),
	})

	rpcService.Start()

	require.LogsContain(t, hook, "listening on port")
	require.LogsContain(t, hook, "You are using an insecure gRPC server")
	assert.NoError(t, rpcService.Stop())
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
			host: "./vanguard.ipc",
		},
		{
			name: "Test IPC socket dial (absolute unix path)",
			host: "/tmp/vanguard.ipc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hook := logTest.NewGlobal()

			chainService := &mock.ChainService{
				Genesis: time.Now(),
			}

			rpcService := NewService(ctx.Context, &Config{
				Host:                tt.host,
				Port:                ctx.String(flags.RPCPort.Name),
				SyncService:         &mockSync.Sync{IsSyncing: false},
				BlockReceiver:       chainService,
				AttestationReceiver: chainService,
				HeadFetcher:         chainService,
				GenesisTimeFetcher:  chainService,
				POWChainService:     &mockPOW.POWChain{},
				StateNotifier:       chainService.StateNotifier(),
			})

			assert.DeepEqual(t, tt.host, rpcService.cfg.Host)
			assert.DeepEqual(t, "4000", rpcService.cfg.Port)

			rpcService.Start()

			require.LogsContain(t, hook, "listening on port")
			assert.NoError(t, rpcService.Stop())
		})
	}
}
