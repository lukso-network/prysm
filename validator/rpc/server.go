package rpc

import (
	"context"
	"errors"
	"github.com/prysmaticlabs/prysm/shared/rpcutil"
	"net"
	"time"

	middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	grpc_opentracing "github.com/grpc-ecosystem/go-grpc-middleware/tracing/opentracing"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	healthpb "github.com/prysmaticlabs/prysm/proto/beacon/rpc/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	pb "github.com/prysmaticlabs/prysm/proto/validator/accounts/v2"
	"github.com/prysmaticlabs/prysm/shared/event"
	"github.com/prysmaticlabs/prysm/shared/logutil"
	"github.com/prysmaticlabs/prysm/shared/rand"
	"github.com/prysmaticlabs/prysm/shared/traceutil"
	"github.com/prysmaticlabs/prysm/validator/accounts/wallet"
	"github.com/prysmaticlabs/prysm/validator/client"
	"github.com/prysmaticlabs/prysm/validator/db"
	"github.com/prysmaticlabs/prysm/validator/keymanager"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/plugin/ocgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
)

// Config options for the gRPC server.
type Config struct {
	ValidatorGatewayHost     string
	ValidatorGatewayPort     int
	ValidatorMonitoringHost  string
	ValidatorMonitoringPort  int
	BeaconClientEndpoint     string
	ClientMaxCallRecvMsgSize int
	ClientGrpcRetries        uint
	ClientGrpcRetryDelay     time.Duration
	ClientGrpcHeaders        []string
	ClientWithCert           string
	Host                     string
	Port                     string
	CertFlag                 string
	KeyFlag                  string
	ValDB                    db.Database
	WalletDir                string
	ValidatorService         *client.ValidatorService
	SyncChecker              client.SyncChecker
	GenesisFetcher           client.GenesisFetcher
	WalletInitializedFeed    *event.Feed
	NodeGatewayEndpoint      string
	Wallet                   *wallet.Wallet
	Keymanager               keymanager.IKeymanager
}

// Server defining a gRPC server for the remote signer API.
type Server struct {
	logsStreamer              logutil.Streamer
	streamLogsBufferSize      int
	beaconChainClient         ethpb.BeaconChainClient
	beaconNodeClient          ethpb.NodeClient
	beaconNodeValidatorClient ethpb.BeaconNodeValidatorClient
	beaconNodeHealthClient    healthpb.HealthClient
	valDB                     db.Database
	ctx                       context.Context
	cancel                    context.CancelFunc
	beaconClientEndpoint      string
	clientMaxCallRecvMsgSize  int
	clientGrpcRetries         uint
	clientGrpcRetryDelay      time.Duration
	clientGrpcHeaders         []string
	clientWithCert            string
	host                      string
	port                      string
	listener                  net.Listener
	keymanager                keymanager.IKeymanager
	withCert                  string
	withKey                   string
	credentialError           error
	grpcServer                *grpc.Server
	jwtKey                    []byte
	validatorService          *client.ValidatorService
	syncChecker               client.SyncChecker
	genesisFetcher            client.GenesisFetcher
	walletDir                 string
	wallet                    *wallet.Wallet
	walletInitializedFeed     *event.Feed
	walletInitialized         bool
	nodeGatewayEndpoint       string
	validatorMonitoringHost   string
	validatorMonitoringPort   int
	validatorGatewayHost      string
	validatorGatewayPort      int
}

// NewServer instantiates a new gRPC server.
func NewServer(ctx context.Context, cfg *Config) *Server {
	ctx, cancel := context.WithCancel(ctx)
	return &Server{
		ctx:                      ctx,
		cancel:                   cancel,
		logsStreamer:             logutil.NewStreamServer(),
		streamLogsBufferSize:     1000, // Enough to handle most bursts of logs in the validator client.
		host:                     cfg.Host,
		port:                     cfg.Port,
		withCert:                 cfg.CertFlag,
		withKey:                  cfg.KeyFlag,
		beaconClientEndpoint:     cfg.BeaconClientEndpoint,
		clientMaxCallRecvMsgSize: cfg.ClientMaxCallRecvMsgSize,
		clientGrpcRetries:        cfg.ClientGrpcRetries,
		clientGrpcRetryDelay:     cfg.ClientGrpcRetryDelay,
		clientGrpcHeaders:        cfg.ClientGrpcHeaders,
		clientWithCert:           cfg.ClientWithCert,
		valDB:                    cfg.ValDB,
		validatorService:         cfg.ValidatorService,
		syncChecker:              cfg.SyncChecker,
		genesisFetcher:           cfg.GenesisFetcher,
		walletDir:                cfg.WalletDir,
		walletInitializedFeed:    cfg.WalletInitializedFeed,
		walletInitialized:        cfg.Wallet != nil,
		wallet:                   cfg.Wallet,
		keymanager:               cfg.Keymanager,
		nodeGatewayEndpoint:      cfg.NodeGatewayEndpoint,
		validatorMonitoringHost:  cfg.ValidatorMonitoringHost,
		validatorMonitoringPort:  cfg.ValidatorMonitoringPort,
		validatorGatewayHost:     cfg.ValidatorGatewayHost,
		validatorGatewayPort:     cfg.ValidatorGatewayPort,
	}
}

// Start the gRPC server.
func (s *Server) Start() {
	// Setup the gRPC server options and TLS configuration.
	address, protocol, err := rpcutil.ResolveRpcAddressAndProtocol(s.host, s.port)
	if err != nil {
		log.Errorf("Could not ResolveRpcAddressAndProtocol in Start() %s: %v", address, err)
	}

	lis, err := net.Listen(protocol, address)
	if err != nil {
		log.Errorf("Could not listen to port in Start() %s: %v", address, err)
	}
	s.listener = lis

	// Register interceptors for metrics gathering as well as our
	// own, custom JWT unary interceptor.
	opts := []grpc.ServerOption{
		grpc.StatsHandler(&ocgrpc.ServerHandler{}),
		grpc.UnaryInterceptor(middleware.ChainUnaryServer(
			recovery.UnaryServerInterceptor(
				recovery.WithRecoveryHandlerContext(traceutil.RecoveryHandlerFunc),
			),
			grpc_prometheus.UnaryServerInterceptor,
			grpc_opentracing.UnaryServerInterceptor(),
			s.JWTInterceptor(),
		)),
	}
	grpc_prometheus.EnableHandlingTimeHistogram()

	if s.withCert != "" && s.withKey != "" {
		creds, err := credentials.NewServerTLSFromFile(s.withCert, s.withKey)
		if err != nil {
			log.WithError(err).Fatal("Could not load TLS keys")
		}
		opts = append(opts, grpc.Creds(creds))
		log.WithFields(logrus.Fields{
			"crt-path": s.withCert,
			"key-path": s.withKey,
		}).Info("Loaded TLS certificates")
	}
	s.grpcServer = grpc.NewServer(opts...)

	// Register a gRPC client to the beacon node.
	if err := s.registerBeaconClient(); err != nil {
		log.WithError(err).Fatal("Could not register beacon chain gRPC client")
	}

	// We create a new, random JWT key upon validator startup.
	jwtKey, err := createRandomJWTKey()
	if err != nil {
		log.WithError(err).Fatal("Could not initialize validator jwt key")
	}
	s.jwtKey = jwtKey

	// Register services available for the gRPC server.
	reflection.Register(s.grpcServer)
	pb.RegisterAuthServer(s.grpcServer, s)
	pb.RegisterWalletServer(s.grpcServer, s)
	pb.RegisterHealthServer(s.grpcServer, s)
	pb.RegisterBeaconServer(s.grpcServer, s)
	pb.RegisterAccountsServer(s.grpcServer, s)
	pb.RegisterSlashingProtectionServer(s.grpcServer, s)

	go func() {
		if s.listener != nil {
			if err := s.grpcServer.Serve(s.listener); err != nil {
				log.Errorf("Could not serve: %v", err)
			}
		}
	}()
	go s.checkUserSignup(s.ctx)
	log.WithField("address", address).Info("gRPC server listening on address")
}

// Stop the gRPC server.
func (s *Server) Stop() error {
	s.cancel()
	if s.listener != nil {
		s.grpcServer.GracefulStop()
		log.Debug("Initiated graceful stop of server")
	}
	return nil
}

// Status returns nil or credentialError.
func (s *Server) Status() error {
	return s.credentialError
}

func createRandomJWTKey() ([]byte, error) {
	r := rand.NewGenerator()
	jwtKey := make([]byte, 32)
	n, err := r.Read(jwtKey)
	if err != nil {
		return nil, err
	}
	if n != len(jwtKey) {
		return nil, errors.New("could not create appropriately sized random JWT key")
	}
	return jwtKey, nil
}
