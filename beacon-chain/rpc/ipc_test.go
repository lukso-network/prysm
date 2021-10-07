package rpc

import (
	"google.golang.org/grpc"
	"net"
	"testing"
	"time"
)

func TestServeIpc(t *testing.T) {
	dialer := func(addr string, t time.Duration) (net.Conn, error) {
		return net.Dial(protocol, addr)
	}

	conn, err := grpc.Dial(sockAddr, grpc.WithInsecure(), grpc.WithDialer(dialer))
	if err != nil {
		log.Fatal(err)
	}
	defer func(conn *grpc.ClientConn) {
		err := conn.Close()
		if err != nil {

		}
	}(conn)
}
