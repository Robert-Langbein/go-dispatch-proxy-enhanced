//go:build linux
// +build linux

// gateway_linux.go
package main

import (
	"fmt"
	"net"
	"os"
	"syscall"
	"unsafe"
)

const (
	SO_ORIGINAL_DST = 80
)

// sockaddr_in structure for IPv4
type sockaddr_in struct {
	sin_family uint16
	sin_port   uint16
	sin_addr   [4]byte
	sin_zero   [8]byte
}

/*
Get original destination for transparent proxy using SO_ORIGINAL_DST (Linux-specific)
*/
func get_original_destination(conn net.Conn) (string, error) {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return "", fmt.Errorf("not a TCP connection")
	}

	// Get the underlying file descriptor
	file, err := tcpConn.File()
	if err != nil {
		return "", fmt.Errorf("failed to get file descriptor: %v", err)
	}
	defer file.Close()

	fd := int(file.Fd())

	// Get original destination using SO_ORIGINAL_DST
	var addr sockaddr_in
	addrLen := uint32(unsafe.Sizeof(addr))

	// Get original destination using raw syscall
	_, _, errno := syscall.Syscall6(
		syscall.SYS_GETSOCKOPT,
		uintptr(fd),
		uintptr(syscall.SOL_IP),
		uintptr(SO_ORIGINAL_DST),
		uintptr(unsafe.Pointer(&addr)),
		uintptr(unsafe.Pointer(&addrLen)),
		0,
	)
	if errno != 0 {
		return "", fmt.Errorf("failed to get original destination: %v", errno)
	}

	// Convert sockaddr_in to IP:port string
	ip := net.IPv4(addr.sin_addr[0], addr.sin_addr[1], addr.sin_addr[2], addr.sin_addr[3])
	port := (uint16(addr.sin_port&0xFF) << 8) | (uint16(addr.sin_port) >> 8) // Convert from network byte order

	return fmt.Sprintf("%s:%d", ip.String(), port), nil
}

/*
Configure network interface for transparent proxy (Linux-specific)
*/
func configure_transparent_interface() error {
	// Enable IP forwarding
	if err := write_sysctl("net.ipv4.ip_forward", "1"); err != nil {
		return fmt.Errorf("failed to enable IP forwarding: %v", err)
	}

	// Enable route_localnet for transparent proxy
	if err := write_sysctl("net.ipv4.conf.all.route_localnet", "1"); err != nil {
		return fmt.Errorf("failed to enable route_localnet: %v", err)
	}

	return nil
}

/*
Write to sysctl parameter
*/
func write_sysctl(param, value string) error {
	path := "/proc/sys/" + param
	return os.WriteFile(path, []byte(value), 0644)
} 