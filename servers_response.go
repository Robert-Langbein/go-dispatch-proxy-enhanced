//go:build !linux
// +build !linux

// servers_response.go
package main

import (
	"log"
	"net"
)

// Legacy server_response function removed - use enhanced_server_response instead

/*
	Enhanced servers response of SOCKS5 for non Linux systems with source IP awareness
*/
func enhanced_server_response(local_conn net.Conn, remote_address string, source_ip string) {
	load_balancer, i := get_enhanced_load_balancer(source_ip)

	// Parse addresses - try IPv4 first, then IPv6
	remote_tcpaddr, err := net.ResolveTCPAddr("tcp4", remote_address)
	if err != nil {
		// Try IPv6 if IPv4 fails
		remote_tcpaddr, err = net.ResolveTCPAddr("tcp6", remote_address)
		if err != nil {
			load_balancer.failure_count++
			log.Printf("[WARN] Invalid remote address %s: %s", remote_address, err)
			local_conn.Write([]byte{5, NETWORK_UNREACHABLE, 0, 1, 0, 0, 0, 0, 0, 0})
			local_conn.Close()
			return
		}
	}
	
	// Parse local IP (without port for non-tunnel mode)
	local_ip := net.ParseIP(load_balancer.address)
	if local_ip == nil {
		load_balancer.failure_count++
		log.Printf("[WARN] Invalid local IP %s", load_balancer.address)
		local_conn.Write([]byte{5, NETWORK_UNREACHABLE, 0, 1, 0, 0, 0, 0, 0, 0})
		local_conn.Close()
		return
	}
	
	// Create local TCP address with IP and port 0 (system chooses port)
	local_tcpaddr := &net.TCPAddr{IP: local_ip, Port: 0}
	
	// Dial with source IP binding
	remote_conn, err := net.DialTCP("tcp4", local_tcpaddr, remote_tcpaddr)

	if err != nil {
		load_balancer.failure_count++
		log.Printf("[WARN] %s -> %s via %s {%s} LB: %d, Source: %s", remote_address, load_balancer.address, load_balancer.address, err, i, source_ip)
		local_conn.Write([]byte{5, NETWORK_UNREACHABLE, 0, 1, 0, 0, 0, 0, 0, 0})
		local_conn.Close()
		return
	}
	load_balancer.success_count++
	log.Printf("[DEBUG] %s -> %s via %s LB: %d, Source: %s", remote_address, load_balancer.address, load_balancer.address, i, source_ip)
	local_conn.Write([]byte{5, SUCCESS, 0, 1, 0, 0, 0, 0, 0, 0})
	
	// Add connection tracking for enhanced function
	conn_id := add_active_connection(local_conn, remote_address, load_balancer, i)
	pipe_connections(local_conn, remote_conn, conn_id)
}
