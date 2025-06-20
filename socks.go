// socks.go
package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"net"
	"time"
)

/*

 */
func client_greeting(conn net.Conn) (byte, []byte, error) {
	buf := make([]byte, 2)

	if nRead, err := conn.Read(buf); err != nil || nRead != len(buf) {
		return 0, nil, errors.New("[WARN] client greeting failed")
	}

	socks_version := buf[0]
	num_auth_methods := buf[1]

	auth_methods := make([]byte, num_auth_methods)

	if nRead, err := conn.Read(auth_methods); err != nil || nRead != int(num_auth_methods) {
		return 0, nil, errors.New("[WARN] client greeting failed")
	}

	return socks_version, auth_methods, nil
}

/*

 */
func servers_choice(conn net.Conn) error {

	if nWrite, err := conn.Write([]byte{5, 0}); err != nil || nWrite != 2 {
		return errors.New("[WARN] servers choice failed")
	}
	return nil
}

/*

 */
func client_conection_request(conn net.Conn) (string, error) {
	header := make([]byte, 4)
	port := make([]byte, 2)
	var address string

	if nRead, err := conn.Read(header); err != nil || nRead != len(header) {
		conn.Write([]byte{5, SERVER_FAILURE, 0, 1, 0, 0, 0, 0, 0, 0})
		conn.Close()
		return "", errors.New("[WARN] client connection request failed")
	}

	socks_version := header[0]
	cmd_code := header[1]
	//	reserved := header[2]
	address_type := header[3]

	if socks_version != 5 {
		conn.Write([]byte{5, SERVER_FAILURE, 0, 1, 0, 0, 0, 0, 0, 0})
		conn.Close()
		return "", errors.New("[WARN] unsupported SOCKS version")
	}

	if cmd_code != CONNECT {
		conn.Write([]byte{5, COMMAND_NOT_SUPPORTED, 0, 1, 0, 0, 0, 0, 0, 0})
		conn.Close()
		return "", errors.New("[WARN] unsupported command code")
	}

	log.Printf("[DEBUG] SOCKS5 request: version=%d, cmd=%d, addr_type=%d", socks_version, cmd_code, address_type)
	
	switch address_type {
	case IPV4:
		ipv4_address := make([]byte, 4)

		if nRead, err := conn.Read(ipv4_address); err != nil || nRead != len(ipv4_address) {
			conn.Write([]byte{5, SERVER_FAILURE, 0, 1, 0, 0, 0, 0, 0, 0})
			conn.Close()
			return "", errors.New("[WARN] client connection request failed")
		}

		if nRead, err := conn.Read(port); err != nil || nRead != len(port) {
			conn.Write([]byte{5, SERVER_FAILURE, 0, 1, 0, 0, 0, 0, 0, 0})
			conn.Close()
			return "", errors.New("[WARN] client connection request failed")
		}
		ipStr := fmt.Sprintf("%d.%d.%d.%d", ipv4_address[0], ipv4_address[1], ipv4_address[2], ipv4_address[3])
		port_num := binary.BigEndian.Uint16(port)
		
		// Log suspicious addresses for debugging
		if ipStr == "0.0.0.0" {
			log.Printf("[DEBUG] Received suspicious IPv4 address: %s:%d", ipStr, port_num)
		}
		
		address = fmt.Sprintf("%s:%d", ipStr, port_num)

	case DOMAIN:
		domain_name_length := make([]byte, 1)

		if nRead, err := conn.Read(domain_name_length); err != nil || nRead != len(domain_name_length) {
			conn.Write([]byte{5, SERVER_FAILURE, 0, 1, 0, 0, 0, 0, 0, 0})
			conn.Close()
			return "", errors.New("[WARN] client connection request failed")
		}

		domain_name := make([]byte, domain_name_length[0])

		if nRead, err := conn.Read(domain_name); err != nil || nRead != len(domain_name) {
			conn.Write([]byte{5, SERVER_FAILURE, 0, 1, 0, 0, 0, 0, 0, 0})
			conn.Close()
			return "", errors.New("[WARN] client connection request failed")
		}

		if nRead, err := conn.Read(port); err != nil || nRead != len(port) {
			conn.Write([]byte{5, SERVER_FAILURE, 0, 1, 0, 0, 0, 0, 0, 0})
			conn.Close()
			return "", errors.New("[WARN] client connection request failed")
		}
		address = fmt.Sprintf("%s:%d", string(domain_name), binary.BigEndian.Uint16(port))

	case IPV6:
		ipv6_address := make([]byte, 16)

		if nRead, err := conn.Read(ipv6_address); err != nil || nRead != len(ipv6_address) {
			conn.Write([]byte{5, SERVER_FAILURE, 0, 1, 0, 0, 0, 0, 0, 0})
			conn.Close()
			return "", errors.New("[WARN] client connection request failed")
		}

		if nRead, err := conn.Read(port); err != nil || nRead != len(port) {
			conn.Write([]byte{5, SERVER_FAILURE, 0, 1, 0, 0, 0, 0, 0, 0})
			conn.Close()
			return "", errors.New("[WARN] client connection request failed")
		}
		
		// Format IPv6 address
		ip := net.IP(ipv6_address)
		ipStr := ip.String()
		
		// Check for invalid IPv6 addresses
		if ipStr == "::" || ipStr == "::1" {
			conn.Write([]byte{5, HOST_UNREACHABLE, 0, 1, 0, 0, 0, 0, 0, 0})
			conn.Close()
			return "", errors.New("[WARN] invalid IPv6 address: " + ipStr)
		}
		
		address = fmt.Sprintf("[%s]:%d", ipStr, binary.BigEndian.Uint16(port))

	default:
		conn.Write([]byte{5, ADDRTYPE_NOT_SUPPORTED, 0, 1, 0, 0, 0, 0, 0, 0})
		conn.Close()
		return "", errors.New("[WARN] unsupported address type")
	}
	return address, nil
}

/*

 */
func handle_socks_connection(conn net.Conn) (string, error) {
	// Set timeout for SOCKS handshake
	conn.SetDeadline(time.Now().Add(10 * time.Second))

	if _, _, err := client_greeting(conn); err != nil {
		if debug_mode {
			log.Println(err)
		}
		return "", err
	}

	if err := servers_choice(conn); err != nil {
		if debug_mode {
			log.Println(err)
		}
		return "", err
	}

	address, err := client_conection_request(conn)
	if err != nil {
		if debug_mode {
			log.Println(err)
		}
		return "", err
	}
	
	// Clear deadline after successful handshake
	conn.SetDeadline(time.Time{})
	return address, nil
}
