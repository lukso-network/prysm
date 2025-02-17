package rpc

import (
	"context"
	"github.com/prysmaticlabs/prysm/shared/rpcutil"
	"net"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_retry "github.com/grpc-ecosystem/go-grpc-middleware/retry"
	grpc_opentracing "github.com/grpc-ecosystem/go-grpc-middleware/tracing/opentracing"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/pkg/errors"
	healthpb "github.com/prysmaticlabs/prysm/proto/beacon/rpc/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	pb "github.com/prysmaticlabs/prysm/proto/validator/accounts/v2"
	"github.com/prysmaticlabs/prysm/shared/grpcutils"
	"github.com/prysmaticlabs/prysm/validator/client"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Initialize a client connect to a beacon node gRPC endpoint.
func (s *Server) registerBeaconClient() error {
	streamInterceptor := grpc.WithStreamInterceptor(middleware.ChainStreamClient(
		grpc_opentracing.StreamClientInterceptor(),
		grpc_prometheus.StreamClientInterceptor,
		grpc_retry.StreamClientInterceptor(),
	))

	beaconRpcAddr, protocol, err := rpcutil.ResolveRpcAddressAndProtocol(s.beaconClientEndpoint, "")
	if err != nil {
		log.Errorf("Could not ResolveRpcAddressAndProtocol in registerBeaconClient() %s: %v", beaconRpcAddr, err)
	}

	dialOpts := client.ConstructDialOptions(
		s.clientMaxCallRecvMsgSize,
		s.clientWithCert,
		s.clientGrpcRetries,
		s.clientGrpcRetryDelay,
		streamInterceptor,
	)

	if dialOpts == nil {
		return errors.New("no dial options for beacon chain gRPC client")
	}

	if "unix" == protocol {
		dialer := func(addr string, t time.Duration) (net.Conn, error) {
			return net.Dial(protocol, addr)
		}

		dialOpts = append(dialOpts, grpc.WithDialer(dialer))
		dialOpts = append(dialOpts, grpc.WithInsecure())
	}

	s.ctx = grpcutils.AppendHeaders(s.ctx, s.clientGrpcHeaders)

	conn, err := grpc.DialContext(s.ctx, beaconRpcAddr, dialOpts...)
	if err != nil {
		return errors.Wrapf(err, "could not dial endpoint: %s", beaconRpcAddr)
	}
	if s.clientWithCert != "" {
		log.Info("Established secure gRPC connection")
	}
	s.beaconChainClient = ethpb.NewBeaconChainClient(conn)
	s.beaconNodeClient = ethpb.NewNodeClient(conn)
	s.beaconNodeHealthClient = healthpb.NewHealthClient(conn)
	s.beaconNodeValidatorClient = ethpb.NewBeaconNodeValidatorClient(conn)
	return nil
}

// GetBeaconStatus retrieves information about the beacon node gRPC connection
// and certain chain metadata, such as the genesis time, the chain head, and the
// deposit contract address.
func (s *Server) GetBeaconStatus(ctx context.Context, _ *empty.Empty) (*pb.BeaconStatusResponse, error) {
	syncStatus, err := s.beaconNodeClient.GetSyncStatus(ctx, &emptypb.Empty{})
	if err != nil {
		return &pb.BeaconStatusResponse{
			BeaconNodeEndpoint: s.nodeGatewayEndpoint,
			Connected:          false,
			Syncing:            false,
		}, nil
	}
	genesis, err := s.beaconNodeClient.GetGenesis(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, err
	}
	genesisTime := uint64(time.Unix(genesis.GenesisTime.Seconds, 0).Unix())
	address := genesis.DepositContractAddress
	chainHead, err := s.beaconChainClient.GetChainHead(ctx, &emptypb.Empty{})
	if err != nil {
		return nil, err
	}
	return &pb.BeaconStatusResponse{
		BeaconNodeEndpoint:     s.beaconClientEndpoint,
		Connected:              true,
		Syncing:                syncStatus.Syncing,
		GenesisTime:            genesisTime,
		DepositContractAddress: address,
		ChainHead:              chainHead,
	}, nil
}

// GetValidatorParticipation is a wrapper around the /eth/v1alpha1 endpoint of the same name.
func (s *Server) GetValidatorParticipation(
	ctx context.Context, req *ethpb.GetValidatorParticipationRequest,
) (*ethpb.ValidatorParticipationResponse, error) {
	return s.beaconChainClient.GetValidatorParticipation(ctx, req)
}

// GetValidatorPerformance is a wrapper around the /eth/v1alpha1 endpoint of the same name.
func (s *Server) GetValidatorPerformance(
	ctx context.Context, req *ethpb.ValidatorPerformanceRequest,
) (*ethpb.ValidatorPerformanceResponse, error) {
	return s.beaconChainClient.GetValidatorPerformance(ctx, req)
}

// GetValidatorBalances is a wrapper around the /eth/v1alpha1 endpoint of the same name.
func (s *Server) GetValidatorBalances(
	ctx context.Context, req *ethpb.ListValidatorBalancesRequest,
) (*ethpb.ValidatorBalances, error) {
	return s.beaconChainClient.ListValidatorBalances(ctx, req)
}

// GetValidators is a wrapper around the /eth/v1alpha1 endpoint of the same name.
func (s *Server) GetValidators(
	ctx context.Context, req *ethpb.ListValidatorsRequest,
) (*ethpb.Validators, error) {
	return s.beaconChainClient.ListValidators(ctx, req)
}

// GetValidatorQueue is a wrapper around the /eth/v1alpha1 endpoint of the same name.
func (s *Server) GetValidatorQueue(
	ctx context.Context, _ *empty.Empty,
) (*ethpb.ValidatorQueue, error) {
	return s.beaconChainClient.GetValidatorQueue(ctx, &emptypb.Empty{})
}

// GetPeers is a wrapper around the /eth/v1alpha1 endpoint of the same name.
func (s *Server) GetPeers(
	ctx context.Context, _ *empty.Empty,
) (*ethpb.Peers, error) {
	return s.beaconNodeClient.ListPeers(ctx, &emptypb.Empty{})
}
