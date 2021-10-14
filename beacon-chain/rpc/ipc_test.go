package rpc

import (
	"context"
	"fmt"
	"go.opencensus.io/plugin/ocgrpc"
	testgrpc "google.golang.org/grpc/interop/grpc_testing"
	"google.golang.org/grpc/metadata"
	"net"
	"os"
	"time"

	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"google.golang.org/grpc"

	"testing"
)

const defaultTestTimeout = 10 * time.Second

var (
	// For headers sent to server:
	testMetadata = metadata.MD{
		"key1":       []string{"value1"},
		"key2":       []string{"value2"},
		"user-agent": []string{fmt.Sprintf("test/0.0.1 grpc-go/%s", grpc.Version)},
	}
	// The id for which the service handler should return error.
	errorID int32 = 32202
)

func TestServeIpc(t *testing.T) {
	ctx := context.Background()

	service := Service{}

	var grpcServerOpts []grpc.ServerOption

	grpcServerOpts = append(grpcServerOpts, grpc.StatsHandler(&ocgrpc.ServerHandler{}))

	service.grpcServer = grpc.NewServer(grpcServerOpts...)

	err := service.serveIpc()
	if err != nil {
		panic(err.Error())
	}

	fileInfo, err := os.Stat(sockAddr)
	if err != nil {
		panic(err.Error())
	}
	assert.NotNil(t, fileInfo)

	dialer := func(addr string, t time.Duration) (net.Conn, error) {
		return net.Dial(protocol, addr)
	}

	dialOpts := []grpc.DialOption{
		grpc.WithInsecure(),
		grpc.WithDialer(dialer),
	}

	conn, err := grpc.DialContext(ctx, sockAddr, dialOpts...)
	defer func(conn *grpc.ClientConn) {
		err = conn.Close()
	}(conn)

	assert.DeepEqual(t, nil, err)
	assert.NotNil(t, conn)

	t.Run("doUnaryCall", func(t *testing.T) {
		var (
			resp *testgrpc.SimpleResponse
			req  *testgrpc.SimpleRequest
			err  error
		)

		ctx, cancel := context.WithTimeout(ctx, defaultTestTimeout)

		defer cancel()

		assert.NotNil(t, ctx)

		tc := testgrpc.NewTestServiceClient(conn)
		assert.NotNil(t, tc)
		req = &testgrpc.SimpleRequest{Payload: idToPayload(errorID)}

		defer cancel()
		resp, err = tc.UnaryCall(metadata.NewOutgoingContext(ctx, testMetadata), req, grpc.WaitForReady(false))
		assert.DeepEqual(t, nil, err)
		assert.NotNil(t, resp)
	})
}

func idToPayload(id int32) *testgrpc.Payload {
	return &testgrpc.Payload{Body: []byte{byte(id), byte(id >> 8), byte(id >> 16), byte(id >> 24)}}
}
