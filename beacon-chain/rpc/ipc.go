package rpc

import (
	"fmt"
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
	ipcListener, err := net.Listen(protocol, sockAddr)
	if err != nil {
		return err
	}

	go func() {
		if s.listener != nil {
			if err := s.grpcServer.Serve(ipcListener); err != nil {
				panic(fmt.Sprintf("Could not serve gRPC: %v", err))
			}
		}
	}()

	return nil
}
