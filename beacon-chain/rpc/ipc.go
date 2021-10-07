package rpc

import (
	"net"
	"os"
)

const (
	protocol = "unix"
	sockAddr = "./vanguard.sock"
)

func (s *Service) cleanupIpc() {
	if _, err := os.Stat(sockAddr); err == nil {
		if err := os.RemoveAll(sockAddr); err != nil {
			log.Fatal(err)
		}
	}
}

func (s *Service) serveIpc() error {
	listener, err := net.Listen(protocol, sockAddr)
	if err != nil {
		return err
	}

	err = s.grpcServer.Serve(listener)
	if err != nil {
		return err
	}

	return nil
}
