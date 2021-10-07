package rpc

import (
	"context"
	"github.com/golang/protobuf/proto"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"net"
	"os"
	"testing"
	"time"
)

func TestServeIpc(t *testing.T) {
	rootCtx := context.Background()
	timeout := 5 * time.Second

	service := Service{}

	var opts []grpc.ServerOption
	service.grpcServer = grpc.NewServer(opts...)

	err := service.serveIpc()
	if err != nil {
		panic(err.Error())
	}

	dialer := func(addr string, t time.Duration) (net.Conn, error) {
		return net.Dial(protocol, addr)
	}

	conn, err := grpc.Dial(sockAddr, grpc.WithInsecure(), grpc.WithDialer(dialer), grpc.WithTimeout(timeout))
	defer func(conn *grpc.ClientConn) {
		err = conn.Close()
	}(conn)

	assert.DeepEqual(t, nil, err)
	assert.NotNil(t, conn)

	fileInfo, err := os.Stat(sockAddr)
	if err != nil {
		panic(err.Error())
	}

	assert.Equal(t, nil, fileInfo)

	t.Run("Check communication", func(t *testing.T) {
		var req, reply proto.Message
		ctx, cancel := context.WithTimeout(rootCtx, timeout)
		defer cancel()

		assert.NotNil(t, ctx)

		if err := conn.Invoke(ctx, "/grpc.testing.TestService/UnimplementedCall", req, reply); err == nil || status.Code(err) != codes.Unimplemented {
			assert.Equal(t, "", err.Error())
		}
	})
}
