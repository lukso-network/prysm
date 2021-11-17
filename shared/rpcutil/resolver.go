package rpcutil

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ResolveRpcAddressAndProtocol returns a RPC address and protocol.
// It can be HTTP/S layer or IPC socket, tcp or unix socket.
func ResolveRpcAddressAndProtocol(host, port string) (address string, protocol string, err error) {
	if port != "" {
		address = fmt.Sprintf("%s:%s", host, port)
	}

	if strings.Contains(address, ".ipc") || port == "" {
		address = host
	}

	u, err := url.Parse(address)
	if err != nil {
		host, _, err = net.SplitHostPort(address)
		if err != nil {
			return address, "", err
		}
	}
	if u != nil {
		host = u.Host
	}

	if net.ParseIP(host) != nil {
		return address, "tcp", nil
	}

	switch u.Scheme {
	case "http", "https":
		return address, "tcp", nil
	case "":
		if len(strings.TrimSpace(u.Path)) == 0 || !strings.Contains(address, ".ipc") {
			return address, "", fmt.Errorf("invalid socket path %q", address)
		}
		return address, "unix", nil
	default:
		return address, protocol, fmt.Errorf("no known network transport layer for address %q", address)
	}
}
