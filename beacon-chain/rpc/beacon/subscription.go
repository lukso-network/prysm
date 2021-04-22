package beacon

import (
	"context"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
)

type Node struct {
	cliCtx   *cli.Context
	ctx      context.Context
	cancel   context.CancelFunc
	lock     sync.RWMutex
	stop     chan struct{} // Channel to wait for termination notifications.
}

// New creates a new node instance, sets up configuration options, and registers
// every required service to the node.
func New(cliCtx *cli.Context) (*Node, error) {
	registry := shared.NewServiceRegistry()
	ctx, cancel := context.WithCancel(cliCtx.Context)
	orchestrator := &Node{
		cliCtx:   cliCtx,
		ctx:      ctx,
		cancel:   cancel,
		services: registry,
		stop:     make(chan struct{}),
	}
	if err := node.registerEpochExtractor(cliCtx); err != nil {
		return nil, err
	}

	if err := node.registerRPCService(cliCtx); err != nil {
		return nil, err
	}
	return node, nil
}

// registerEpochExtractor
func (o *Node) registerEpochExtractor(cliCtx *cli.Context) error {
	pandoraHttpUrl := cliCtx.String(cmd.PandoraRPCEndpoint.Name)
	vanguardHttpUrl := cliCtx.String(cmd.VanguardRPCEndpoint.Name)
	genesisTime := cliCtx.Uint64(cmd.GenesisTime.Name)

	svc, err := epochextractor.NewService(o.ctx, pandoraHttpUrl, vanguardHttpUrl, genesisTime)
	if err != nil {
		return nil
	}
	return o.services.RegisterService(svc)
}

// register RPC server
func (o *Node) registerRPCService(cliCtx *cli.Context) error {
	log.Info("Registering rpc server")

	var epochExtractorService *epochextractor.Service
	if err := o.services.FetchService(&epochExtractorService); err != nil {
		return err
	}

	var ipcapiURL string
	if cliCtx.String(cmd.IPCPathFlag.Name) != "" {
		ipcFilePath := cliCtx.String(cmd.IPCPathFlag.Name)
		ipcapiURL = fileutil.IpcEndpoint(filepath.Join(ipcFilePath, "node.ipc"), "")
	}

	httpEnable := cliCtx.Bool(cmd.HTTPEnabledFlag.Name)
	httpListenAddr := cliCtx.String(cmd.HTTPListenAddrFlag.Name)
	httpPort := cliCtx.Int(cmd.HTTPPortFlag.Name)
	wsEnable := cliCtx.Bool(cmd.WSEnabledFlag.Name)
	wsListenerAddr := cliCtx.String(cmd.WSListenAddrFlag.Name)
	wsPort := cliCtx.Int(cmd.WSPortFlag.Name)

	svc, err := rpc.NewService(o.ctx, &rpc.Config{
		EpochExpractor: epochExtractorService,
		IPCPath:        ipcapiURL,
		HTTPEnable:     httpEnable,
		HTTPHost:       httpListenAddr,
		HTTPPort:       httpPort,
		WSEnable:       wsEnable,
		WSHost:         wsListenerAddr,
		WSPort:         wsPort,
	})
	if err != nil {
		return nil
	}
	return o.services.RegisterService(svc)
}

// Start the BeaconNode and kicks off every registered service.
func (o *Node) Start() {
	o.lock.Lock()

	o.services.StartAll()

	stop := o.stop
	o.lock.Unlock()

	go func() {
		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(sigc)
		<-sigc
		go o.Close()
		for i := 10; i > 0; i-- {
			<-sigc
			if i > 1 {
				log.WithField("times", i-1).Info("Already shutting down, interrupt more to panic")
			}
		}
		panic("Panic closing the beacon node")
	}()

	// Wait for stop channel to be closed.
	<-stop
}

// Close handles graceful shutdown of the system.
func (b *Node) Close() {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.services.StopAll()
	b.cancel()
	close(b.stop)
}
