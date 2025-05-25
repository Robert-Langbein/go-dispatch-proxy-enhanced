// servers_response_linux.go
package main

import (
	"fmt"
	"log"
	"net"
	"syscall"
)

/*
	Implements servers response of SOCKS5 for linux systems
*/
func server_response(local_conn net.Conn, remote_address string) {
	load_balancer, i := get_load_balancer()
	local_tcpaddr, _ := net.ResolveTCPAddr("tcp4", load_balancer.address)

	dialer := net.Dialer{
		LocalAddr: local_tcpaddr,
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				// NOTE: Run with root or use setcap to allow interface binding
				// sudo setcap cap_net_raw=eip ./go-dispatch-proxy
				if err := syscall.BindToDevice(int(fd), load_balancer.iface); err != nil {
					log.Println("[WARN] Couldn't bind to interface", load_balancer.iface, "LB:", i)
				}
			})
		},
	}

	remote_conn, err := dialer.Dial("tcp4", remote_address)
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
	Enhanced servers response of SOCKS5 for linux systems with source IP awareness
*/
func enhanced_server_response(local_conn net.Conn, remote_address string, source_ip string) {
	load_balancer, i := get_enhanced_load_balancer(source_ip)
	local_tcpaddr, _ := net.ResolveTCPAddr("tcp4", load_balancer.address)

	dialer := net.Dialer{
		LocalAddr: local_tcpaddr,
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				// NOTE: Run with root or use setcap to allow interface binding
				// sudo setcap cap_net_raw=eip ./go-dispatch-proxy
				if err := syscall.BindToDevice(int(fd), load_balancer.iface); err != nil {
					log.Printf("[WARN] Couldn't bind to interface %s LB: %d, Source: %s", load_balancer.iface, i, source_ip)
				}
			})
		},
	}

	remote_conn, err := dialer.Dial("tcp4", remote_address)
	if err != nil {
		load_balancer.failure_count++
		log.Printf("[WARN] %s -> %s via %s {%s} LB: %d, Source: %s", remote_address, load_balancer.address, load_balancer.iface, err, i, source_ip)
		local_conn.Write([]byte{5, NETWORK_UNREACHABLE, 0, 1, 0, 0, 0, 0, 0, 0})
		local_conn.Close()
		return
	}

	load_balancer.success_count++
	log.Printf("[DEBUG] %s -> %s via %s LB: %d, Source: %s", remote_address, load_balancer.address, load_balancer.iface, i, source_ip)
	local_conn.Write([]byte{5, SUCCESS, 0, 1, 0, 0, 0, 0, 0, 0})
	pipe_connections(local_conn, remote_conn)
}
