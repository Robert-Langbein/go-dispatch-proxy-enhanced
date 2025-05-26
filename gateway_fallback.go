//go:build !linux
// +build !linux

// gateway_fallback.go
package main

import (
	"fmt"
	"net"
)

/*
Get original destination for transparent proxy (fallback for non-Linux systems)
*/
func get_original_destination(conn net.Conn) (string, error) {
	// For non-Linux systems, we can't use SO_ORIGINAL_DST
	// This is a limitation - transparent proxy mode won't work properly
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return "", fmt.Errorf("not a TCP connection")
	}
	
	// Return remote address as fallback (this won't work for transparent proxy)
	remoteAddr := tcpConn.RemoteAddr().String()
	return remoteAddr, fmt.Errorf("transparent proxy not supported on this platform")
}

/*
Configure network interface for transparent proxy (fallback for non-Linux systems)
*/
func configure_transparent_interface() error {
	return fmt.Errorf("transparent proxy configuration not supported on this platform")
} 