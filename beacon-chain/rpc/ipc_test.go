package rpc

import (
	"context"
	"net"
	"os"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"google.golang.org/grpc"

	"testing"
)

func TestServeIpc(t *testing.T) {
	ctx := context.Background()
	timeout := 5 * time.Second

	service := Service{}

	grpcServerOpts := []grpc.ServerOption{
		grpc.ChainStreamInterceptor(),
	}
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

	t.Run("Check communication", func(t *testing.T) {
		var req, reply proto.Message
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		assert.NotNil(t, ctx)

		err := conn.Invoke(ctx, "/ethereum.eth.v1alpha1.BeaconChain/GetBeaconConfig", req, reply)
		if err != nil {
			t.Fatalf("error is = %v", err)
		}
	})
}
