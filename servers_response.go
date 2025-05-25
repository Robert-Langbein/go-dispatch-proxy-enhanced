//go:build !linux
// +build !linux

// servers_response.go
package main

import (
	"fmt"
	"log"
	"net"
)

/*
	Implements servers response of SOCKS5 for non Linux systems
*/
func server_response(local_conn net.Conn, remote_address string) {
	load_balancer, i := get_load_balancer()

	local_tcpaddr, _ := net.ResolveTCPAddr("tcp4", load_balancer.address)
	remote_tcpaddr, _ := net.ResolveTCPAddr("tcp4", remote_address)
	remote_conn, err := net.DialTCP("tcp4", local_tcpaddr, remote_tcpaddr)

	if err != nil {
		load_balancer.failure_count++
		log.Println("[WARN]", remote_address, "->", load_balancer.address, fmt.Sprintf("{%s}", err), "LB:", i)
		local_conn.Write([]byte{5, NETWORK_UNREACHABLE, 0, 1, 0, 0, 0, 0, 0, 0})
		local_conn.Close()
		return
	}
	load_balancer.success_count++
	log.Println("[DEBUG]", remote_address, "->", load_balancer.address, "LB:", i)
	local_conn.Write([]byte{5, SUCCESS, 0, 1, 0, 0, 0, 0, 0, 0})
	pipe_connections(local_conn, remote_conn)
}

/*
	Enhanced servers response of SOCKS5 for non Linux systems with source IP awareness
*/
func enhanced_server_response(local_conn net.Conn, remote_address string, source_ip string) {
	load_balancer, i := get_enhanced_load_balancer(source_ip)

	local_tcpaddr, _ := net.ResolveTCPAddr("tcp4", load_balancer.address)
	remote_tcpaddr, _ := net.ResolveTCPAddr("tcp4", remote_address)
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
	pipe_connections(local_conn, remote_conn)
}
