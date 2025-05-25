//go:build !linux
// +build !linux

// servers_response.go
package main

import (
	"log"
	"net"
	"time"
)

// Legacy server_response function removed - use enhanced_server_response instead

/*
	Enhanced servers response of SOCKS5 for non Linux systems with source IP awareness
*/
func enhanced_server_response(local_conn net.Conn, remote_address string, source_ip string) {
	load_balancer, i := get_enhanced_load_balancer(source_ip)

	// Parse local IP (without port for non-tunnel mode)
	local_ip := net.ParseIP(load_balancer.address)
	if local_ip == nil {
		load_balancer.failure_count++
		log.Printf("[WARN] Invalid local IP %s", load_balancer.address)
		local_conn.Write([]byte{5, NETWORK_UNREACHABLE, 0, 1, 0, 0, 0, 0, 0, 0})
		local_conn.Close()
		return
	}
	
	// Determine if we're using IPv4 or IPv6 for local interface
	var network string
	var local_tcpaddr *net.TCPAddr
	
	if local_ip.To4() != nil {
		// Local IP is IPv4, prefer IPv4 connections
		network = "tcp4"
		local_tcpaddr = &net.TCPAddr{IP: local_ip, Port: 0}
	} else {
		// Local IP is IPv6, prefer IPv6 connections
		network = "tcp6"
		local_tcpaddr = &net.TCPAddr{IP: local_ip, Port: 0}
	}
	
	// Simple validation - skip complex DNS resolution for now
	if debug_mode {
		log.Printf("[DEBUG] Processing %s using %s network for source %s", remote_address, network, source_ip)
	}
	
	// Let the dialer handle DNS resolution with timeout - much simpler and more reliable
	
	// Create dialer with local address binding and aggressive timeouts
	dialer := &net.Dialer{
		LocalAddr: local_tcpaddr,
		Timeout:   5 * time.Second,  // 5 second total timeout (DNS + connect)
		KeepAlive: -1,              // Disable keep-alive to avoid hanging connections
	}
	
	// Dial to remote address with timeout
	remote_conn, err := dialer.Dial(network, remote_address)
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
