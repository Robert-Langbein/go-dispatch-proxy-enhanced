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
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

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
			lb_list[lb_index].last_traffic_update = time.Now()
	mutex.Unlock()
		}
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
	var lhost = flag.String("lhost", "127.0.0.1", "The host to listen for SOCKS connection")
	var lport = flag.Int("lport", 8080, "The local port to listen for SOCKS connection")
	var detect = flag.Bool("list", false, "Shows the available addresses for dispatching (non-tunnelling mode only)")
	var tunnel = flag.Bool("tunnel", false, "Use tunnelling mode (acts as a transparent load balancing proxy)")
	var quiet = flag.Bool("quiet", false, "disable logs")
	var stats = flag.Bool("stats", false, "Show load balancer statistics and exit")
	var configFile = flag.String("config", "source_ip_rules.json", "Configuration file for source IP rules")
	var webPort = flag.Int("web", 0, "Enable web GUI on specified port (0 = disabled, 80 = recommended)")
	var debug = flag.Bool("debug", false, "Enable detailed debug logging")

	flag.Parse()
	
	// Set custom config file path
	config_file = *configFile
	
	if *detect {
		detect_interfaces()
		return
	}
	
	if *stats {
		//Parse remaining string to get addresses of load balancers for stats
		parse_load_balancers(flag.Args(), *tunnel)
		load_source_ip_rules()
		get_load_balancer_stats()
		return
	}

	// Enable debug logging if requested
	if *debug {
		debug_mode = true
		log.Printf("[DEBUG] Debug mode enabled")
	}

	// Disable timestamp in log messages
	log.SetFlags(log.Flags() &^ (log.Ldate | log.Ltime))

	// Check for valid IP
	if net.ParseIP(*lhost).To4() == nil {
		log.Fatal("[FATAL] Invalid host ", *lhost)
	}

	// Check for valid port
	if *lport < 1 || *lport > 65535 {
		log.Fatal("[FATAL] Invalid port ", *lport)
	}

	//Parse remaining string to get addresses of load balancers
	parse_load_balancers(flag.Args(), *tunnel)

	// Load source IP rules configuration
	load_source_ip_rules()

	// Start web server if enabled
	if *webPort > 0 {
		log.Printf("[INFO] Starting web server on port %d", *webPort)
		startWebServer(*webPort)
	}

	local_bind_address := fmt.Sprintf("%s:%d", *lhost, *lport)

	// Start local server
	l, err := net.Listen("tcp4", local_bind_address)
	if err != nil {
		log.Fatalln("[FATAL] Could not start local server on ", local_bind_address)
	}
	log.Println("[INFO] Enhanced SOCKS5 proxy with source IP load balancing started on ", local_bind_address)
	log.Printf("[INFO] Load balancing %d interfaces with enhanced features", len(lb_list))
	
	// Print enhanced load balancer information
	for i, lb := range lb_list {
		custom_rules := len(lb.source_ip_rules)
		status := "enabled"
		if !lb.enabled {
			status = "disabled"
		}
		log.Printf("[INFO] LB %d: %s (%s) - Default ratio: %d, Custom rules: %d, Status: %s", 
			i+1, lb.address, lb.iface, lb.contention_ratio, custom_rules, status)
	}
	
	if *debug {
		log.Printf("[DEBUG] Available network interfaces:")
		detect_interfaces()
	}
	
	defer l.Close()
	defer stopWebServer()

	if (*quiet) {
		log.SetOutput(io.Discard)
	}

	mutex = &sync.Mutex{}
	
	// Start cleanup goroutine for old connections
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
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
			go handle_connection(conn, *tunnel)
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
