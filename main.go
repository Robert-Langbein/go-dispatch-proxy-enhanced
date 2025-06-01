// main.go
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Gateway mode configuration
type gateway_config struct {
	enabled             bool
	transparent_port    int
	dns_port           int
	gateway_ip         string
	subnet_cidr        string
	iptables_rules     []string
	nat_interface      string
	backup_rules_file  string
	auto_configure     bool
	dhcp_range_start   string
	dhcp_range_end     string
}

// Global gateway configuration
var gateway_cfg gateway_config

type source_ip_rule struct {
	SourceIP         string `json:"source_ip"`
	ContentionRatio  int    `json:"contention_ratio"`
	Description      string `json:"description"`
}

// Real-time connection tracking structure
type active_connection struct {
	ID              string    `json:"id"`
	SourceIP        string    `json:"source_ip"`
	SourcePort      int       `json:"source_port"`
	DestinationIP   string    `json:"destination_ip"`
	DestinationPort int       `json:"destination_port"`
	LoadBalancer    string    `json:"load_balancer"`
	LBIndex         int       `json:"lb_index"`
	StartTime       time.Time `json:"start_time"`
	LastActivity    time.Time `json:"last_activity"`
	BytesIn         int64     `json:"bytes_in"`
	BytesOut        int64     `json:"bytes_out"`
	Status          string    `json:"status"` // "active", "closing", "closed"
	Protocol        string    `json:"protocol"`
	ProcessInfo     string    `json:"process_info,omitempty"` // Optional process information
}

// Traffic statistics for real-time monitoring
type traffic_stats struct {
	BytesPerSecond    int64 `json:"bytes_per_second"`
	ConnectionsPerMin int64 `json:"connections_per_min"`
	LastUpdate        time.Time `json:"last_update"`
}

// Traffic sample for real-time monitoring
type TrafficSample struct {
	Timestamp    time.Time `json:"timestamp"`
	TotalBytesIn int64     `json:"total_bytes_in"`
	TotalBytesOut int64    `json:"total_bytes_out"`
}

// Client traffic tracking
type ClientTrafficStats struct {
	SourceIP         string          `json:"source_ip"`
	BytesInTotal     int64           `json:"bytes_in_total"`
	BytesOutTotal    int64           `json:"bytes_out_total"`
	BytesInPerSecond int64           `json:"bytes_in_per_second"`
	BytesOutPerSecond int64          `json:"bytes_out_per_second"`
	TrafficSamples   []TrafficSample `json:"-"` // Don't expose in JSON
	LastUpdate       time.Time       `json:"last_update"`
	TrafficMutex     sync.RWMutex    `json:"-"` // Don't expose in JSON
}

type enhanced_load_balancer struct {
	address             string
	iface               string
	contention_ratio    int
	current_connections int
	// Enhanced features for source IP specific weighting
	source_ip_rules     map[string]source_ip_rule  // source_ip -> custom rule
	source_ip_counters  map[string]int             // source_ip -> current connections for this LB
	total_connections   int                        // total connections handled by this LB
	success_count       int                        // successful connections
	failure_count       int                        // failed connections
	enabled             bool                       // whether this LB is enabled
	
	// Real-time traffic monitoring
	traffic_stats       traffic_stats              // current traffic statistics
	bytes_transferred   int64                      // total bytes transferred
	last_traffic_update time.Time                  // last traffic update timestamp
	
	// Enhanced per-LB traffic tracking
	bytes_in_total      int64                      // total bytes received through this LB
	bytes_out_total     int64                      // total bytes sent through this LB
	bytes_in_per_second int64                      // current bytes/sec in through this LB
	bytes_out_per_second int64                     // current bytes/sec out through this LB
	traffic_samples     []TrafficSample            // recent traffic samples for this LB
	traffic_mutex       sync.RWMutex               // mutex for traffic samples
}

// The load balancer used in the previous connection (global round robin)
var lb_index int = 0

// Source IP specific load balancer indices
var source_lb_indices map[string]int

// List of all load balancers (enhanced version)
var lb_list []enhanced_load_balancer

// Mutex to serialize access to function get_load_balancer
var mutex *sync.Mutex

// Configuration file for source IP rules
var config_file string = "source_ip_rules.json"

// Global debug flag
var debug_mode bool = false
var quiet_mode bool = false

// Real-time connection tracking
var active_connections map[string]*active_connection
var connection_mutex *sync.RWMutex
var connection_history []active_connection
var max_connections int = 500 // Performance limit

// Global traffic statistics
var global_bytes_in int64
var global_bytes_out int64
var global_start_time time.Time

// Performance counters
var total_data_transferred int64

// Global client traffic tracking
var client_traffic_stats map[string]*ClientTrafficStats
var client_traffic_mutex sync.RWMutex

// Connection timeout and limits
const (
	connection_timeout = 30 * time.Second
	idle_timeout      = 5 * time.Minute
	max_goroutines    = 1000 // Limit concurrent goroutines
	handshake_timeout = 10 * time.Second
	dns_timeout      = 8 * time.Second
)

var active_goroutines int64

// Connection rate limiting
var (
	connection_semaphore = make(chan struct{}, max_goroutines/2) // Limit concurrent connections
	last_cleanup time.Time
)

func init() {
	active_connections = make(map[string]*active_connection)
	connection_mutex = &sync.RWMutex{}
	connection_history = make([]active_connection, 0, max_connections)
	global_start_time = time.Now()
	
	// Initialize client traffic tracking
	client_traffic_stats = make(map[string]*ClientTrafficStats)
}

/*
Load source IP rules from configuration file
*/
func load_source_ip_rules() {
	if _, err := os.Stat(config_file); os.IsNotExist(err) {
		log.Println("[INFO] No source IP rules configuration file found, using defaults")
		return
	}

	file, err := os.Open(config_file)
	if err != nil {
		log.Printf("[WARN] Could not open source IP rules file: %v", err)
		return
	}
	defer file.Close()

	var rules_config map[string]map[string]source_ip_rule
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&rules_config); err != nil {
		log.Printf("[WARN] Could not parse source IP rules file: %v", err)
		return
	}

	// Apply rules to load balancers
	for i := range lb_list {
		lb_addr := lb_list[i].address
		if rules, exists := rules_config[lb_addr]; exists {
			if lb_list[i].source_ip_rules == nil {
				lb_list[i].source_ip_rules = make(map[string]source_ip_rule)
			}
			for source_ip, rule := range rules {
				lb_list[i].source_ip_rules[source_ip] = rule
				log.Printf("[INFO] Applied rule for %s -> %s: ratio=%d", source_ip, lb_addr, rule.ContentionRatio)
			}
		}
	}
}

/*
Save source IP rules to configuration file
*/
func save_source_ip_rules() {
	rules_config := make(map[string]map[string]source_ip_rule)
	
	for _, lb := range lb_list {
		if len(lb.source_ip_rules) > 0 {
			rules_config[lb.address] = lb.source_ip_rules
		}
	}

	file, err := os.Create(config_file)
	if err != nil {
		log.Printf("[WARN] Could not create source IP rules file: %v", err)
		return
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(rules_config); err != nil {
		log.Printf("[WARN] Could not save source IP rules: %v", err)
	} else {
		log.Println("[INFO] Source IP rules saved successfully")
	}
}

/*
Get source IP from connection
*/
func get_source_ip(conn net.Conn) string {
	if addr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
		return addr.IP.String()
	}
	return ""
}

/*
Get effective contention ratio for a source IP and load balancer
*/
func get_effective_contention_ratio(lb *enhanced_load_balancer, source_ip string) int {
	if rule, exists := lb.source_ip_rules[source_ip]; exists {
		return rule.ContentionRatio
	}
	return lb.contention_ratio
}

/*
Get a load balancer according to contention ratio with enhanced source IP awareness
*/
func get_enhanced_load_balancer(source_ip string, params ...interface{}) (*enhanced_load_balancer, int) {
	var _bitset *big.Int
	if len(params) > 0 {
		seed := -1
		for _, p := range params {
			switch v := p.(type) {
			case int:
				seed = v
			case *big.Int:
				_bitset = v
			}
		}
		if seed < 0 || seed >= len(lb_list) || _bitset == nil {
			seed = -1
			_bitset = nil
		}
		log.Printf("[DEBUG] Try to get different load balancer of %d for source %s", seed, source_ip)
	}

	mutex.Lock()
	defer mutex.Unlock()

	// Initialize source IP specific indices if needed
	if source_lb_indices == nil {
		source_lb_indices = make(map[string]int)
	}

	// Get source-specific index or use global index
	current_index, exists := source_lb_indices[source_ip]
	if !exists {
		current_index = lb_index
		source_lb_indices[source_ip] = current_index
	}

	// Handle bitset for failed load balancers
	if _bitset != nil {
		for {
			if _bitset.Bit(current_index) != 0 {
				lb := &lb_list[current_index]
				if lb.source_ip_counters == nil {
					lb.source_ip_counters = make(map[string]int)
				}
				lb.source_ip_counters[source_ip] = 0
				current_index = (current_index + 1) % len(lb_list)
				source_lb_indices[source_ip] = current_index
			} else {
				break
			}
		}
	}

	// Find next available enabled load balancer
	start_index := current_index
	for {
		lb := &lb_list[current_index]
		if lb.enabled {
			break
		}
		current_index = (current_index + 1) % len(lb_list)
		if current_index == start_index {
			// All load balancers are disabled
			log.Printf("[WARN] All load balancers are disabled for source %s", source_ip)
			return &lb_list[0], 0 // Return first LB as fallback
		}
	}

	lb := &lb_list[current_index]
	
	// Initialize counters if needed
	if lb.source_ip_counters == nil {
		lb.source_ip_counters = make(map[string]int)
	}

	// Get effective contention ratio for this source IP
	effective_ratio := get_effective_contention_ratio(lb, source_ip)
	
	// Increment counters
	lb.source_ip_counters[source_ip]++
	lb.current_connections++
	lb.total_connections++

	ilb := current_index

	// Check if we need to move to next load balancer
	if lb.source_ip_counters[source_ip] >= effective_ratio {
		lb.source_ip_counters[source_ip] = 0
		current_index = (current_index + 1) % len(lb_list)
		source_lb_indices[source_ip] = current_index
	}

	// Update global index as well
	if lb.current_connections >= lb.contention_ratio {
		lb.current_connections = 0
		lb_index = (lb_index + 1) % len(lb_list)
	}

	log.Printf("[DEBUG] Selected LB %d (%s) for source %s, effective ratio: %d", ilb, lb.address, source_ip, effective_ratio)
	return lb, ilb
}

// Legacy get_load_balancer function removed - use get_enhanced_load_balancer instead

/*
Generate unique connection ID
*/
func generate_connection_id() string {
	return fmt.Sprintf("conn_%d_%d", time.Now().UnixNano(), len(active_connections))
}

/*
Add new active connection to tracking
*/
func add_active_connection(conn net.Conn, remote_addr string, lb *enhanced_load_balancer, lb_index int) string {
	connection_mutex.Lock()
	defer connection_mutex.Unlock()
	
	// Clean up old connections if we hit the limit
	if len(active_connections) >= max_connections {
		cleanup_old_connections()
	}
	
	conn_id := generate_connection_id()
	source_ip := get_source_ip(conn)
	source_port := 0
	dest_ip := ""
	dest_port := 0
	
	// Extract port from source
	if addr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
		source_port = addr.Port
	}
	
	// Extract destination info
	if strings.Contains(remote_addr, ":") {
		parts := strings.Split(remote_addr, ":")
		dest_ip = parts[0]
		if len(parts) > 1 {
			dest_port, _ = strconv.Atoi(parts[1])
		}
	}
	
	active_conn := &active_connection{
		ID:              conn_id,
		SourceIP:        source_ip,
		SourcePort:      source_port,
		DestinationIP:   dest_ip,
		DestinationPort: dest_port,
		LoadBalancer:    lb.address,
		LBIndex:         lb_index,
		StartTime:       time.Now(),
		LastActivity:    time.Now(),
		BytesIn:         0,
		BytesOut:        0,
		Status:          "active",
		Protocol:        "TCP",
		ProcessInfo:     "", // Could be enhanced with process detection
	}
	
	active_connections[conn_id] = active_conn
	log.Printf("[DEBUG] Added active connection %s: %s:%d -> %s:%d via LB%d", 
		conn_id, source_ip, source_port, dest_ip, dest_port, lb_index+1)
	
	return conn_id
}

/*
Update connection traffic statistics
*/
func update_connection_traffic(conn_id string, bytes_in, bytes_out int64) {
	connection_mutex.Lock()
	defer connection_mutex.Unlock()
	
	if conn, exists := active_connections[conn_id]; exists {
		conn.BytesIn += bytes_in
		conn.BytesOut += bytes_out
		conn.LastActivity = time.Now()
		
		// Update global counters
		atomic.AddInt64(&global_bytes_in, bytes_in)
		atomic.AddInt64(&global_bytes_out, bytes_out)
		atomic.AddInt64(&total_data_transferred, bytes_in+bytes_out)
		
		// Update load balancer traffic stats
		if lb_index := conn.LBIndex; lb_index >= 0 && lb_index < len(lb_list) {
			mutex.Lock()
			lb_list[lb_index].bytes_transferred += bytes_in + bytes_out
			lb_list[lb_index].bytes_in_total += bytes_in
			lb_list[lb_index].bytes_out_total += bytes_out
			lb_list[lb_index].last_traffic_update = time.Now()
			mutex.Unlock()
		}
		
		// Update client traffic stats
		updateClientTrafficStats(conn.SourceIP, bytes_in, bytes_out)
	}
}

/*
Remove active connection from tracking
*/
func remove_active_connection(conn_id string) {
	connection_mutex.Lock()
	defer connection_mutex.Unlock()
	
	if conn, exists := active_connections[conn_id]; exists {
		conn.Status = "closed"
		
		// Add to history with limit check
		if len(connection_history) >= max_connections {
			// Remove oldest entry
			connection_history = connection_history[1:]
		}
		connection_history = append(connection_history, *conn)
		
		delete(active_connections, conn_id)
		log.Printf("[DEBUG] Removed active connection %s after %v", 
			conn_id, time.Since(conn.StartTime))
	}
}

/*
Cleanup old connections (performance optimization)
*/
func cleanup_old_connections() {
	connection_mutex.Lock()
	defer connection_mutex.Unlock()
	
	cutoff := time.Now().Add(-idle_timeout) // Remove connections older than idle_timeout
	removed := 0
	
	for id, conn := range active_connections {
		if conn.LastActivity.Before(cutoff) || conn.Status == "closed" {
			// Add to history before removing
			if len(connection_history) >= max_connections {
				connection_history = connection_history[1:]
			}
			conn.Status = "expired"
			connection_history = append(connection_history, *conn)
			
			delete(active_connections, id)
			removed++
		}
	}
	
	// Also clean up history if it gets too large
	if len(connection_history) > max_connections*2 {
		connection_history = connection_history[len(connection_history)-max_connections:]
	}
	
	if removed > 0 && debug_mode {
		log.Printf("[DEBUG] Cleaned up %d old connections", removed)
	}
}

/*
Get current active connections (with filters)
*/
func get_active_connections(source_filter, destination_filter string, limit int) []active_connection {
	connection_mutex.RLock()
	defer connection_mutex.RUnlock()
	
	result := make([]active_connection, 0, len(active_connections))
	count := 0
	
	for _, conn := range active_connections {
		// Apply filters
		if source_filter != "" && !strings.Contains(conn.SourceIP, source_filter) {
			continue
		}
		if destination_filter != "" && !strings.Contains(conn.DestinationIP, destination_filter) {
			continue
		}
		
		result = append(result, *conn)
		count++
		
		if limit > 0 && count >= limit {
			break
		}
	}
	
	return result
}

/*
Custom io.Copy with traffic monitoring and timeout
*/
func monitored_copy(dst io.Writer, src io.Reader, conn_id string, direction string) (int64, error) {
	buffer := make([]byte, 32*1024) // 32KB buffer for performance
	var total int64
	
	// Set read deadline for timeout detection
	if conn, ok := src.(net.Conn); ok {
		conn.SetReadDeadline(time.Now().Add(idle_timeout))
	}
	
	for {
		nr, er := src.Read(buffer)
		if nr > 0 {
			// Reset deadline on successful read
			if conn, ok := src.(net.Conn); ok {
				conn.SetReadDeadline(time.Now().Add(idle_timeout))
			}
			
			nw, ew := dst.Write(buffer[0:nr])
			if nw < 0 || nr < nw {
				nw = 0
				if ew == nil {
					ew = fmt.Errorf("invalid write result")
				}
			}
			
			total += int64(nw)
			
			// Update traffic statistics
			if direction == "in" {
				update_connection_traffic(conn_id, int64(nw), 0)
			} else {
				update_connection_traffic(conn_id, 0, int64(nw))
			}
			
			if ew != nil {
				return total, ew
			}
			if nr != nw {
				return total, io.ErrShortWrite
			}
		}
		if er != nil {
			if er != io.EOF {
				return total, er
			}
			break
		}
	}
	return total, nil
}

/*
Enhanced pipe connections with traffic monitoring
*/
func pipe_connections(local_conn, remote_conn net.Conn, conn_id string) {
	var wg sync.WaitGroup
	wg.Add(2)
	
	// Ensure connections are only closed once
	var closeOnce sync.Once
	
	closeConnections := func() {
		if local_conn != nil {
			local_conn.Close()
		}
		if remote_conn != nil {
			remote_conn.Close()
		}
		remove_active_connection(conn_id)
	}

	// Copy from local to remote (outbound)
	go func() {
		defer wg.Done()
		defer closeOnce.Do(closeConnections)
		
		_, err := monitored_copy(remote_conn, local_conn, conn_id, "out")
		if err != nil && debug_mode {
			log.Printf("[DEBUG] Connection %s outbound error: %v", conn_id, err)
		}
	}()

	// Copy from remote to local (inbound)
	go func() {
		defer wg.Done()
		defer closeOnce.Do(closeConnections)
		
		_, err := monitored_copy(local_conn, remote_conn, conn_id, "in")
		if err != nil && debug_mode {
			log.Printf("[DEBUG] Connection %s inbound error: %v", conn_id, err)
		}
	}()
	
	// Wait for both goroutines to complete
	wg.Wait()
}

/*
Handle connections in tunnel mode with enhanced load balancing
*/
func handle_tunnel_connection(conn net.Conn) {
	source_ip := get_source_ip(conn)
	load_balancer, i := get_enhanced_load_balancer(source_ip)
	var _bitset *big.Int
	complete := 1 == len(lb_list)

retry:
	remote_addr, _ := net.ResolveTCPAddr("tcp4", load_balancer.address)
	remote_conn, err := net.DialTCP("tcp4", nil, remote_addr)

	if err != nil {
		load_balancer.failure_count++
		log.Printf("[WARN] %s -> %s {%s} LB: %d, Source: %s", load_balancer.address, remote_addr.String(), err, i, source_ip)

		if !complete && _bitset == nil {
			bits := make([]byte, (len(lb_list)+7)/8)
			_bitset = new(big.Int).SetBytes(bits)
		}

		if !complete {
			_bitset.SetBit(_bitset, i, 1)

			// Check if all balancers are used
			mask := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), uint(len(lb_list))), big.NewInt(1))
			complete = new(big.Int).And(_bitset, mask).Cmp(mask) == 0
		}

		if !complete {
			load_balancer, i = get_enhanced_load_balancer(source_ip, i, _bitset)
			goto retry
		}

		log.Printf("[WARN] All load balancers failed for source %s", source_ip)
		conn.Close()
		return
	}

	load_balancer.success_count++
	log.Printf("[DEBUG] Tunnelled %s to %s LB: %d", source_ip, load_balancer.address, i)
	
	// Add connection tracking for tunnel mode
	conn_id := add_active_connection(conn, load_balancer.address, load_balancer, i)
	pipe_connections(conn, remote_conn, conn_id)
}

/*
Calls the appropriate handle_connections based on tunnel mode with enhanced features
*/
func handle_connection(conn net.Conn, tunnel bool) {
	// Check goroutine limit
	if atomic.LoadInt64(&active_goroutines) >= max_goroutines {
		log.Printf("[WARN] Maximum goroutines reached, rejecting connection")
		conn.Close()
		return
	}
	
	// Acquire connection semaphore (non-blocking)
	select {
	case connection_semaphore <- struct{}{}:
		defer func() { <-connection_semaphore }()
	default:
		if debug_mode {
			log.Printf("[DEBUG] Connection semaphore full, rejecting connection")
		}
		conn.Close()
		return
	}
	
	atomic.AddInt64(&active_goroutines, 1)
	defer atomic.AddInt64(&active_goroutines, -1)
	
	// Set initial connection timeout for handshake
	conn.SetDeadline(time.Now().Add(handshake_timeout))
	
	source_ip := get_source_ip(conn)
	
	if tunnel {
		handle_tunnel_connection(conn)
	} else {
		// Handle SOCKS connection with immediate processing
		if debug_mode {
			log.Printf("[DEBUG] Starting SOCKS handshake for %s", source_ip)
		}
		
		if address, err := handle_socks_connection(conn); err == nil {
			if debug_mode {
				log.Printf("[DEBUG] SOCKS handshake successful for %s -> %s", source_ip, address)
			}
			
			// Start server response in separate goroutine to prevent blocking
			go func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("[ERROR] Panic in server response for %s: %v", source_ip, r)
						conn.Close()
					}
				}()
				
				if debug_mode {
					log.Printf("[DEBUG] Starting enhanced_server_response for %s -> %s", source_ip, address)
				}
				enhanced_server_response(conn, address, source_ip)
			}()
		} else {
			if debug_mode {
				log.Printf("[DEBUG] SOCKS handshake failed for %s: %v", source_ip, err)
			}
			conn.Close()
		}
	}
}

/*
Detect the addresses which can  be used for dispatching in non-tunnelling mode.
Alternate to ipconfig/ifconfig
*/
func detect_interfaces() {
	fmt.Println("--- Listing the available adresses for dispatching")
	ifaces, _ := net.Interfaces()

	for _, iface := range ifaces {
		if (iface.Flags&net.FlagUp == net.FlagUp) && (iface.Flags&net.FlagLoopback != net.FlagLoopback) {
			addrs, _ := iface.Addrs()
			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					if ipnet.IP.To4() != nil {
						fmt.Printf("[+] %s, IPv4:%s\n", iface.Name, ipnet.IP.String())
					}
				}
			}
		}
	}

}

/*
Gets the interface associated with the IP
*/
func get_iface_from_ip(ip string) string {
	ifaces, _ := net.Interfaces()

	for _, iface := range ifaces {
		if (iface.Flags&net.FlagUp == net.FlagUp) && (iface.Flags&net.FlagLoopback != net.FlagLoopback) {
			addrs, _ := iface.Addrs()
			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					if ipnet.IP.To4() != nil {
						if ipnet.IP.String() == ip {
							return iface.Name
						}
					}
				}
			}
		}
	}
	return ""
}

/*
Parses the command line arguements to obtain the list of load balancers
*/
func parse_load_balancers(args []string, tunnel bool) {
	if len(args) == 0 {
		log.Fatal("[FATAL] Please specify one or more load balancers")
	}

	lb_list = make([]enhanced_load_balancer, flag.NArg())

	for idx, a := range args {
		splitted := strings.Split(a, "@")
		iface := ""
		// IP address of a Fully Qualified Domain Name of the load balancer
		var lb_ip_or_fqdn string
		var lb_port int
		var err error

		if tunnel {
			ip_or_fqdn_port := strings.Split(splitted[0], ":")
			if len(ip_or_fqdn_port) != 2 {
				log.Fatal("[FATAL] Invalid address specification ", splitted[0])
				return
			}

			lb_ip_or_fqdn = ip_or_fqdn_port[0]
			lb_port, err = strconv.Atoi(ip_or_fqdn_port[1])
			if err != nil || lb_port <= 0 || lb_port > 65535 {
				log.Fatal("[FATAL] Invalid port ", splitted[0])
				return
			}

		} else {
			lb_ip_or_fqdn = splitted[0]
			lb_port = 0
		}

		// FQDN not supported for tunnel modes
		if !tunnel && net.ParseIP(lb_ip_or_fqdn).To4() == nil {
			log.Fatal("[FATAL] Invalid address ", lb_ip_or_fqdn)
		}

		var cont_ratio int = 1
		if len(splitted) > 1 {
			cont_ratio, err = strconv.Atoi(splitted[1])
			if err != nil || cont_ratio <= 0 {
				log.Fatal("[FATAL] Invalid contention ratio for ", lb_ip_or_fqdn)
			}
		}

		// Obtaining the interface name of the load balancer IP's doesn't make sense in tunnel mode
		if !tunnel {
			iface = get_iface_from_ip(lb_ip_or_fqdn)
			if iface == "" {
				log.Fatal("[FATAL] IP address not associated with an interface ", lb_ip_or_fqdn)
			}
		}

		slbport := ""
		if tunnel {
			slbport = ":" + strconv.Itoa(lb_port)
		}

		log.Printf("[INFO] Load balancer %d: %s%s, contention ratio: %d\n", idx+1, lb_ip_or_fqdn, slbport, cont_ratio)
		
		// Store address differently for tunnel vs non-tunnel mode
		var address string
		if tunnel {
			address = fmt.Sprintf("%s:%d", lb_ip_or_fqdn, lb_port)
		} else {
			address = lb_ip_or_fqdn // Only IP for non-tunnel mode
		}
		
		lb_list[idx] = enhanced_load_balancer{address: address, iface: iface, contention_ratio: cont_ratio, current_connections: 0, source_ip_rules: make(map[string]source_ip_rule), source_ip_counters: make(map[string]int), total_connections: 0, success_count: 0, failure_count: 0, enabled: true}
	}
}

/*
Main function
*/
func main() {
	// Only webgui flag for initial setup - all settings managed via database
	var webPort = flag.Int("webgui", 8090, "Web GUI port for initial setup (saved to database on first run)")
	flag.Parse()

	log.Printf("[INFO] Go Dispatch Proxy Enhanced v3.0 - Database Edition")
	log.Printf("[INFO] Starting with default settings - configuration via WebUI")

	// Initialize database first
	if err := initDatabase(); err != nil {
		log.Fatalf("[FATAL] Database initialization failed: %v", err)
	}
	defer closeDatabase()

	// Initialize webPort from command line flag if this is first startup
	if err := initializeWebPortFromFlag(*webPort); err != nil {
		log.Fatalf("[FATAL] Failed to initialize webPort from flag: %v", err)
	}

	// Load all configuration from database (webPort is now stored there)
	if err := syncSettingsFromDatabase(); err != nil {
		log.Fatalf("[FATAL] Failed to load settings from database: %v", err)
	}

	if err := syncGatewayConfigFromDatabase(); err != nil {
		log.Fatalf("[FATAL] Failed to load gateway config from database: %v", err)
	}

	// Apply loaded settings to global variables
	debug_mode = currentSettings.DebugMode
	quiet_mode = currentSettings.QuietMode
	config_file = currentSettings.ConfigFile

	// Initialize gateway configuration from database
	gateway_cfg = gateway_config{
		enabled:           currentSettings.GatewayMode,
		transparent_port:  currentSettings.TransparentPort,
		dns_port:         currentSettings.DNSPort,
		gateway_ip:       currentSettings.GatewayIP,
		subnet_cidr:      currentSettings.SubnetCIDR,
		nat_interface:    currentSettings.NATInterface,
		auto_configure:   currentSettings.AutoConfig,
		dhcp_range_start: currentSettings.DHCPStart,
		dhcp_range_end:   currentSettings.DHCPEnd,
		backup_rules_file: "iptables_backup.rules",
		iptables_rules:   make([]string, 0),
	}

	mutex = &sync.Mutex{}
	
	// Load load balancers from database
	if err := loadLoadBalancersFromDatabase(); err != nil {
		log.Printf("[WARN] Failed to load load balancers from database: %v", err)
	}

	// Disable timestamp in log messages if quiet mode
	if quiet_mode {
		log.SetOutput(io.Discard)
	} else {
		log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))
	}

	// Enable debug logging if configured
	if debug_mode {
		log.Printf("[DEBUG] Debug mode enabled from database settings")
	}

	// Initialize gateway mode if enabled
	if gateway_cfg.enabled {
		if err := initialize_gateway_mode(); err != nil {
			log.Printf("[ERROR] Failed to initialize gateway mode: %v", err)
			// Don't exit, just log the error
		}
	}

	// Initialize web server settings (no longer needs all the parameters)
	InitializeSettingsFromDatabase()
	
	// Start web server (always enabled by default now)
	if currentSettings.WebPort > 0 {
		log.Printf("[INFO] Starting web server on port %d", currentSettings.WebPort)
		startWebServer(currentSettings.WebPort)
	}

	local_bind_address := fmt.Sprintf("%s:%d", currentSettings.ListenHost, currentSettings.ListenPort)

	// Start local server
	l, err := net.Listen("tcp4", local_bind_address)
	if err != nil {
		log.Fatalf("[FATAL] Could not start local server on %s: %v", local_bind_address, err)
	}
	
	log.Printf("[INFO] Enhanced SOCKS5 proxy started on %s", local_bind_address)
	log.Printf("[INFO] Web GUI available at http://localhost:%d", currentSettings.WebPort)
	log.Printf("[INFO] Load balancing %d interfaces", len(lb_list))
	
	// Print load balancer information
	for i, lb := range lb_list {
		custom_rules := len(lb.source_ip_rules)
		status := "enabled"
		if !lb.enabled {
			status = "disabled"
		}
		log.Printf("[INFO] LB %d: %s (%s) - Ratio: %d, Rules: %d, Status: %s", 
			i+1, lb.address, lb.iface, lb.contention_ratio, custom_rules, status)
	}
	
	if debug_mode {
		log.Printf("[DEBUG] Available network interfaces:")
		detect_interfaces()
	}
	
	defer l.Close()
	defer stopWebServer()
	defer cleanup_gateway_mode()
	
	// Start periodic database sync and cleanup
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				// Save statistics snapshot
				if err := saveStatisticsSnapshot(); err != nil {
					log.Printf("[WARN] Failed to save statistics: %v", err)
				}
				// Sync load balancers to database
				syncLoadBalancersToDatabase()
				// Cleanup old connections
				cleanup_old_connections()
			}
		}
	}()
	
	// Start resource monitoring
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				goroutines := atomic.LoadInt64(&active_goroutines)
				connections := len(active_connections)
				if debug_mode {
					log.Printf("[DEBUG] Resources: %d goroutines, %d active connections", goroutines, connections)
				}
				if goroutines > max_goroutines*8/10 { // 80% threshold
					log.Printf("[WARN] High goroutine usage: %d/%d", goroutines, max_goroutines)
				}
			}
		}
	}()
	
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Println("[WARN] Could not accept connection")
		} else {
			go handle_connection(conn, currentSettings.TunnelMode)
		}
	}
}

/*
Add or update a source IP rule for a specific load balancer
*/
func add_source_ip_rule(lb_address string, source_ip string, contention_ratio int, description string) bool {
	mutex.Lock()
	defer mutex.Unlock()
	
	for i := range lb_list {
		if lb_list[i].address == lb_address {
			if lb_list[i].source_ip_rules == nil {
				lb_list[i].source_ip_rules = make(map[string]source_ip_rule)
			}
			
			lb_list[i].source_ip_rules[source_ip] = source_ip_rule{
				SourceIP:        source_ip,
				ContentionRatio: contention_ratio,
				Description:     description,
			}
			
			log.Printf("[INFO] Added source IP rule: %s -> %s (ratio: %d) - %s", 
				source_ip, lb_address, contention_ratio, description)
			
			// Save to file
			save_source_ip_rules()
			return true
		}
	}
	
	log.Printf("[WARN] Load balancer %s not found", lb_address)
	return false
}

/*
Remove a source IP rule for a specific load balancer
*/
func remove_source_ip_rule(lb_address string, source_ip string) bool {
	mutex.Lock()
	defer mutex.Unlock()
	
	for i := range lb_list {
		if lb_list[i].address == lb_address {
			if lb_list[i].source_ip_rules != nil {
				if _, exists := lb_list[i].source_ip_rules[source_ip]; exists {
					delete(lb_list[i].source_ip_rules, source_ip)
					log.Printf("[INFO] Removed source IP rule: %s -> %s", source_ip, lb_address)
					
					// Save to file
					save_source_ip_rules()
					return true
				}
			}
			log.Printf("[WARN] Source IP rule not found: %s -> %s", source_ip, lb_address)
			return false
		}
	}
	
	log.Printf("[WARN] Load balancer %s not found", lb_address)
	return false
}

/*
Enable or disable a load balancer
*/
func set_load_balancer_status(lb_address string, enabled bool) bool {
	mutex.Lock()
	defer mutex.Unlock()
	
	for i := range lb_list {
		if lb_list[i].address == lb_address {
			lb_list[i].enabled = enabled
			status := "enabled"
			if !enabled {
				status = "disabled"
			}
			log.Printf("[INFO] Load balancer %s %s", lb_address, status)
			return true
		}
	}
	
	log.Printf("[WARN] Load balancer %s not found", lb_address)
	return false
}

/*
Get statistics for all load balancers
*/
func get_load_balancer_stats() {
	mutex.Lock()
	defer mutex.Unlock()
	
	fmt.Println("\n--- Load Balancer Statistics ---")
	for i, lb := range lb_list {
		status := "enabled"
		if !lb.enabled {
			status = "disabled"
		}
		
		success_rate := 0.0
		total := lb.success_count + lb.failure_count
		if total > 0 {
			success_rate = float64(lb.success_count) / float64(total) * 100
		}
		
		fmt.Printf("LB %d: %s (%s) - Status: %s\n", i+1, lb.address, lb.iface, status)
		fmt.Printf("  Default ratio: %d, Total connections: %d\n", lb.contention_ratio, lb.total_connections)
		fmt.Printf("  Success: %d, Failures: %d, Success rate: %.2f%%\n", 
			lb.success_count, lb.failure_count, success_rate)
		
		if len(lb.source_ip_rules) > 0 {
			fmt.Printf("  Custom source IP rules (%d):\n", len(lb.source_ip_rules))
			for source_ip, rule := range lb.source_ip_rules {
				fmt.Printf("    %s -> ratio: %d (%s)\n", source_ip, rule.ContentionRatio, rule.Description)
			}
		}
		
		if len(lb.source_ip_counters) > 0 {
			fmt.Printf("  Current source IP connections:\n")
			for source_ip, count := range lb.source_ip_counters {
				fmt.Printf("    %s: %d\n", source_ip, count)
			}
		}
		fmt.Println()
	}
}

/*
Initialize gateway mode with iptables rules and transparent proxy
*/
func initialize_gateway_mode() error {
	log.Printf("[INFO] Initializing gateway mode on %s with subnet %s", gateway_cfg.gateway_ip, gateway_cfg.subnet_cidr)
	
	// Check if running as root (required for iptables and transparent proxy)
	if os.Geteuid() != 0 {
		return fmt.Errorf("gateway mode requires root privileges")
	}
	
	// Auto-detect NAT interface if not specified
	if gateway_cfg.nat_interface == "" {
		iface, err := detect_nat_interface()
		if err != nil {
			return fmt.Errorf("failed to auto-detect NAT interface: %v", err)
		}
		gateway_cfg.nat_interface = iface
		log.Printf("[INFO] Auto-detected NAT interface: %s", iface)
	}
	
	// Backup existing iptables rules
	if err := backup_iptables_rules(); err != nil {
		log.Printf("[WARN] Failed to backup iptables rules: %v", err)
	}
	
	// Configure iptables rules for transparent proxy
	if gateway_cfg.auto_configure {
		if err := configure_iptables_rules(); err != nil {
			return fmt.Errorf("failed to configure iptables rules: %v", err)
		}
	}
	
	// Start transparent proxy server
	go start_transparent_proxy()
	
	// Start DNS server for gateway mode
	go start_gateway_dns_server()
	
	log.Printf("[INFO] Gateway mode initialized successfully")
	log.Printf("[INFO] Clients should use %s as their default gateway", gateway_cfg.gateway_ip)
	log.Printf("[INFO] Transparent proxy listening on port %d", gateway_cfg.transparent_port)
	log.Printf("[INFO] DNS server listening on port %d", gateway_cfg.dns_port)
	
	return nil
}

/*
Detect the best NAT interface for gateway mode
*/
func detect_nat_interface() (string, error) {
	// Get default route interface
	cmd := exec.Command("ip", "route", "show", "default")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	
	// Parse output to find interface
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "default via") {
			parts := strings.Fields(line)
			for i, part := range parts {
				if part == "dev" && i+1 < len(parts) {
					return parts[i+1], nil
				}
			}
		}
	}
	
	return "", fmt.Errorf("no default route found")
}

/*
Backup existing iptables rules
*/
func backup_iptables_rules() error {
	cmd := exec.Command("iptables-save")
	output, err := cmd.Output()
	if err != nil {
		return err
	}
	
	return os.WriteFile(gateway_cfg.backup_rules_file, output, 0644)
}

/*
Configure iptables rules for transparent proxy and NAT
*/
func configure_iptables_rules() error {
	log.Printf("[INFO] Configuring iptables rules for gateway mode")
	
	rules := []string{
		// Enable IP forwarding
		"echo 1 > /proc/sys/net/ipv4/ip_forward",
		
		// Create custom chain for transparent proxy
		"iptables -t nat -N DISPATCH_PROXY 2>/dev/null || true",
		"iptables -t mangle -N DISPATCH_PROXY 2>/dev/null || true",
		
		// Redirect TCP traffic to transparent proxy (except for proxy itself)
		fmt.Sprintf("iptables -t nat -A DISPATCH_PROXY -p tcp --dport 1:65535 -j REDIRECT --to-port %d", gateway_cfg.transparent_port),
		
		// Redirect traffic from clients to transparent proxy
		fmt.Sprintf("iptables -t nat -A PREROUTING -s %s -p tcp --dport 1:65535 -j DISPATCH_PROXY", gateway_cfg.subnet_cidr),
		
		// NAT for outgoing traffic
		fmt.Sprintf("iptables -t nat -A POSTROUTING -s %s -o %s -j MASQUERADE", gateway_cfg.subnet_cidr, gateway_cfg.nat_interface),
		
		// Allow forwarding for our subnet
		fmt.Sprintf("iptables -A FORWARD -s %s -j ACCEPT", gateway_cfg.subnet_cidr),
		fmt.Sprintf("iptables -A FORWARD -d %s -j ACCEPT", gateway_cfg.subnet_cidr),
		
		// DNS redirection to our DNS server
		fmt.Sprintf("iptables -t nat -A PREROUTING -s %s -p udp --dport 53 -j REDIRECT --to-port %d", gateway_cfg.subnet_cidr, gateway_cfg.dns_port),
		fmt.Sprintf("iptables -t nat -A PREROUTING -s %s -p tcp --dport 53 -j REDIRECT --to-port %d", gateway_cfg.subnet_cidr, gateway_cfg.dns_port),
	}
	
	gateway_cfg.iptables_rules = rules
	
	for _, rule := range rules {
		if strings.HasPrefix(rule, "echo") {
			// Handle sysctl commands
			cmd := exec.Command("sh", "-c", rule)
			if err := cmd.Run(); err != nil {
				log.Printf("[WARN] Failed to execute: %s - %v", rule, err)
			}
		} else {
			// Handle iptables commands
			parts := strings.Fields(rule)
			cmd := exec.Command(parts[0], parts[1:]...)
			if err := cmd.Run(); err != nil {
				log.Printf("[WARN] Failed to execute: %s - %v", rule, err)
			}
		}
	}
	
	log.Printf("[INFO] iptables rules configured successfully")
	return nil
}

/*
Start transparent proxy server for gateway mode
*/
func start_transparent_proxy() {
	addr := fmt.Sprintf(":%d", gateway_cfg.transparent_port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Printf("[ERROR] Failed to start transparent proxy on %s: %v", addr, err)
		return
	}
	defer listener.Close()
	
	log.Printf("[INFO] Transparent proxy started on %s", addr)
	
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("[WARN] Failed to accept transparent proxy connection: %v", err)
			continue
		}
		
		go handle_transparent_connection(conn)
	}
}

/*
Handle transparent proxy connections
*/
func handle_transparent_connection(conn net.Conn) {
	defer conn.Close()
	
	// Get original destination using SO_ORIGINAL_DST
	originalDest, err := get_original_destination(conn)
	if err != nil {
		log.Printf("[WARN] Failed to get original destination: %v", err)
		return
	}
	
	source_ip := get_source_ip(conn)
	log.Printf("[DEBUG] Transparent proxy: %s -> %s", source_ip, originalDest)
	
	// Use enhanced load balancer for transparent connections
	load_balancer, i := get_enhanced_load_balancer(source_ip)
	
	// Create connection to target through selected load balancer
	local_tcpaddr, _ := net.ResolveTCPAddr("tcp4", load_balancer.address)
	dialer := net.Dialer{
		LocalAddr: local_tcpaddr,
		Timeout:   10 * time.Second,
	}
	
	remote_conn, err := dialer.Dial("tcp", originalDest)
	if err != nil {
		load_balancer.failure_count++
		log.Printf("[WARN] Transparent proxy failed to connect to %s via %s: %v", originalDest, load_balancer.address, err)
		return
	}
	defer remote_conn.Close()
	
	load_balancer.success_count++
	log.Printf("[DEBUG] Transparent proxy connected: %s -> %s via LB%d (%s)", source_ip, originalDest, i, load_balancer.address)
	
	// Add connection tracking
	conn_id := add_active_connection(conn, originalDest, load_balancer, i)
	pipe_connections(conn, remote_conn, conn_id)
}

/*
Start DNS server for gateway mode
*/
func start_gateway_dns_server() {
	// Simple DNS forwarder - forwards to system DNS or configured DNS servers
	addr := fmt.Sprintf(":%d", gateway_cfg.dns_port)
	
	// Start UDP DNS server
	go func() {
		udpAddr, err := net.ResolveUDPAddr("udp", addr)
		if err != nil {
			log.Printf("[ERROR] Failed to resolve UDP DNS address: %v", err)
			return
		}
		
		conn, err := net.ListenUDP("udp", udpAddr)
		if err != nil {
			log.Printf("[ERROR] Failed to start UDP DNS server: %v", err)
			return
		}
		defer conn.Close()
		
		log.Printf("[INFO] DNS server (UDP) started on %s", addr)
		
		for {
			buffer := make([]byte, 512)
			n, clientAddr, err := conn.ReadFromUDP(buffer)
			if err != nil {
				continue
			}
			
			go handle_dns_query(conn, clientAddr, buffer[:n])
		}
	}()
	
	// Start TCP DNS server
	go func() {
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			log.Printf("[ERROR] Failed to start TCP DNS server: %v", err)
			return
		}
		defer listener.Close()
		
		log.Printf("[INFO] DNS server (TCP) started on %s", addr)
		
		for {
			conn, err := listener.Accept()
			if err != nil {
				continue
			}
			
			go handle_dns_tcp_connection(conn)
		}
	}()
}

/*
Handle DNS queries by forwarding to upstream DNS servers
*/
func handle_dns_query(conn *net.UDPConn, clientAddr *net.UDPAddr, query []byte) {
	// Forward to system DNS (8.8.8.8 as fallback)
	upstreamDNS := "8.8.8.8:53"
	
	// Try to get system DNS from /etc/resolv.conf
	if systemDNS := get_system_dns(); systemDNS != "" {
		upstreamDNS = systemDNS + ":53"
	}
	
	// Forward query to upstream DNS
	upstreamConn, err := net.Dial("udp", upstreamDNS)
	if err != nil {
		return
	}
	defer upstreamConn.Close()
	
	upstreamConn.Write(query)
	
	response := make([]byte, 512)
	n, err := upstreamConn.Read(response)
	if err != nil {
		return
	}
	
	// Send response back to client
	conn.WriteToUDP(response[:n], clientAddr)
}

/*
Handle TCP DNS connections
*/
func handle_dns_tcp_connection(conn net.Conn) {
	defer conn.Close()
	
	// Read DNS query length (TCP DNS has 2-byte length prefix)
	lengthBytes := make([]byte, 2)
	if _, err := conn.Read(lengthBytes); err != nil {
		return
	}
	
	queryLength := int(lengthBytes[0])<<8 | int(lengthBytes[1])
	query := make([]byte, queryLength)
	if _, err := conn.Read(query); err != nil {
		return
	}
	
	// Forward to upstream DNS
	upstreamDNS := "8.8.8.8:53"
	if systemDNS := get_system_dns(); systemDNS != "" {
		upstreamDNS = systemDNS + ":53"
	}
	
	upstreamConn, err := net.Dial("tcp", upstreamDNS)
	if err != nil {
		return
	}
	defer upstreamConn.Close()
	
	// Send query with length prefix
	upstreamConn.Write(lengthBytes)
	upstreamConn.Write(query)
	
	// Read response
	responseLengthBytes := make([]byte, 2)
	if _, err := upstreamConn.Read(responseLengthBytes); err != nil {
		return
	}
	
	responseLength := int(responseLengthBytes[0])<<8 | int(responseLengthBytes[1])
	response := make([]byte, responseLength)
	if _, err := upstreamConn.Read(response); err != nil {
		return
	}
	
	// Send response back to client
	conn.Write(responseLengthBytes)
	conn.Write(response)
}

/*
Get system DNS server from /etc/resolv.conf
*/
func get_system_dns() string {
	content, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return ""
	}
	
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "nameserver ") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}
	
	return ""
}

/*
Cleanup gateway mode (restore iptables rules)
*/
func cleanup_gateway_mode() error {
	if !gateway_cfg.enabled {
		return nil
	}
	
	log.Printf("[INFO] Cleaning up gateway mode...")
	
	// Restore iptables rules from backup
	if _, err := os.Stat(gateway_cfg.backup_rules_file); err == nil {
		cmd := exec.Command("iptables-restore", gateway_cfg.backup_rules_file)
		if err := cmd.Run(); err != nil {
			log.Printf("[WARN] Failed to restore iptables rules: %v", err)
		} else {
			log.Printf("[INFO] iptables rules restored from backup")
		}
	}
	
	// Remove custom chains
	exec.Command("iptables", "-t", "nat", "-F", "DISPATCH_PROXY").Run()
	exec.Command("iptables", "-t", "nat", "-X", "DISPATCH_PROXY").Run()
	exec.Command("iptables", "-t", "mangle", "-F", "DISPATCH_PROXY").Run()
	exec.Command("iptables", "-t", "mangle", "-X", "DISPATCH_PROXY").Run()
	
	return nil
}

/*
Update client traffic statistics for real-time monitoring
*/
func updateClientTrafficStats(sourceIP string, bytesIn, bytesOut int64) {
	client_traffic_mutex.Lock()
	defer client_traffic_mutex.Unlock()
	
	// Get or create client stats
	clientStats, exists := client_traffic_stats[sourceIP]
	if !exists {
		clientStats = &ClientTrafficStats{
			SourceIP:       sourceIP,
			TrafficSamples: make([]TrafficSample, 0, 10),
			LastUpdate:     time.Now(),
		}
		client_traffic_stats[sourceIP] = clientStats
	}
	
	// Update totals
	clientStats.BytesInTotal += bytesIn
	clientStats.BytesOutTotal += bytesOut
	clientStats.LastUpdate = time.Now()
	
	// Add traffic sample for speed calculation
	now := time.Now()
	clientStats.TrafficSamples = append(clientStats.TrafficSamples, TrafficSample{
		Timestamp:    now,
		TotalBytesIn: clientStats.BytesInTotal,
		TotalBytesOut: clientStats.BytesOutTotal,
	})
	
	// Keep only last 10 samples for performance
	if len(clientStats.TrafficSamples) > 10 {
		clientStats.TrafficSamples = clientStats.TrafficSamples[len(clientStats.TrafficSamples)-10:]
	}
	
	// Calculate current speed based on last few samples
	if len(clientStats.TrafficSamples) >= 2 {
		latest := clientStats.TrafficSamples[len(clientStats.TrafficSamples)-1]
		earliest := clientStats.TrafficSamples[0]
		
		timeDiff := latest.Timestamp.Sub(earliest.Timestamp).Seconds()
		if timeDiff > 0 {
			bytesInDiff := latest.TotalBytesIn - earliest.TotalBytesIn
			bytesOutDiff := latest.TotalBytesOut - earliest.TotalBytesOut
			
			clientStats.BytesInPerSecond = int64(float64(bytesInDiff) / timeDiff)
			clientStats.BytesOutPerSecond = int64(float64(bytesOutDiff) / timeDiff)
		}
	}
}

/*
Update load balancer traffic statistics for real-time monitoring
*/
func updateLoadBalancerTrafficStats() {
	mutex.Lock()
	defer mutex.Unlock()
	
	for i := range lb_list {
		lb := &lb_list[i]
		
		// Add traffic sample for speed calculation
		now := time.Now()
		
		lb.traffic_mutex.Lock()
		lb.traffic_samples = append(lb.traffic_samples, TrafficSample{
			Timestamp:    now,
			TotalBytesIn: lb.bytes_in_total,
			TotalBytesOut: lb.bytes_out_total,
		})
		
		// Keep only last 10 samples for performance
		if len(lb.traffic_samples) > 10 {
			lb.traffic_samples = lb.traffic_samples[len(lb.traffic_samples)-10:]
		}
		
		// Calculate current speed based on last few samples
		if len(lb.traffic_samples) >= 2 {
			latest := lb.traffic_samples[len(lb.traffic_samples)-1]
			earliest := lb.traffic_samples[0]
			
			timeDiff := latest.Timestamp.Sub(earliest.Timestamp).Seconds()
			if timeDiff > 0 {
				bytesInDiff := latest.TotalBytesIn - earliest.TotalBytesIn
				bytesOutDiff := latest.TotalBytesOut - earliest.TotalBytesOut
				
				lb.bytes_in_per_second = int64(float64(bytesInDiff) / timeDiff)
				lb.bytes_out_per_second = int64(float64(bytesOutDiff) / timeDiff)
			}
		}
		lb.traffic_mutex.Unlock()
	}
}

/*
Get client traffic statistics for a specific source IP
*/
func getClientTrafficStats(sourceIP string) *ClientTrafficStats {
	client_traffic_mutex.RLock()
	defer client_traffic_mutex.RUnlock()
	
	if stats, exists := client_traffic_stats[sourceIP]; exists {
		// Return a copy to avoid race conditions
		statsCopy := *stats
		return &statsCopy
	}
	
	return &ClientTrafficStats{
		SourceIP: sourceIP,
		LastUpdate: time.Now(),
	}
}


