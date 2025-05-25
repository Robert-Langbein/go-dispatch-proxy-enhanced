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
	
	// Validate remote address resolution with detailed logging
	if debug_mode {
		log.Printf("[DEBUG] Resolving %s using %s network for source %s", remote_address, network, source_ip)
	}
	
	resolved_addr, err := net.ResolveTCPAddr(network, remote_address)
	if err != nil {
		// If same family fails and we're on IPv4, try IPv6
		if network == "tcp4" {
			if debug_mode {
				log.Printf("[DEBUG] IPv4 resolution failed for %s, trying IPv6", remote_address)
			}
			resolved_addr, err = net.ResolveTCPAddr("tcp6", remote_address)
			if err == nil {
				// Switch to IPv6 but still prefer local IPv4 binding if possible
				if debug_mode {
					log.Printf("[DEBUG] %s resolved to IPv6 (%s), but using IPv4 local binding for %s", 
						remote_address, resolved_addr.String(), source_ip)
				}
				// Use general "tcp" to let Go handle the family conversion
				network = "tcp"
			}
		}
		
		if err != nil {
			load_balancer.failure_count++
			log.Printf("[WARN] Cannot resolve remote address %s: %s (Source: %s)", remote_address, err, source_ip)
			local_conn.Write([]byte{5, NETWORK_UNREACHABLE, 0, 1, 0, 0, 0, 0, 0, 0})
			local_conn.Close()
			return
		}
	} else {
		if debug_mode {
			log.Printf("[DEBUG] Successfully resolved %s to %s using %s", remote_address, resolved_addr.String(), network)
		}
	}
	
	// Create dialer with local address binding
	dialer := &net.Dialer{
		LocalAddr: local_tcpaddr,
	}
	
	// Dial to remote address
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
