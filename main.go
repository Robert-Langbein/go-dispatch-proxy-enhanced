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
)

type source_ip_rule struct {
	SourceIP         string `json:"source_ip"`
	ContentionRatio  int    `json:"contention_ratio"`
	Description      string `json:"description"`
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

/*
Backward compatibility: Get a load balancer according to contention ratio (legacy function)
*/
func get_load_balancer(params ...interface{}) (*enhanced_load_balancer, int) {
	return get_enhanced_load_balancer("", params...)
}

/*
Joins the local and remote connections together
*/
func pipe_connections(local_conn, remote_conn net.Conn) {
	go func() {
		defer remote_conn.Close()
		defer local_conn.Close()
		_, err := io.Copy(remote_conn, local_conn)
		if err != nil {
			return
		}
	}()

	go func() {
		defer remote_conn.Close()
		defer local_conn.Close()
		_, err := io.Copy(local_conn, remote_conn)
		if err != nil {
			return
		}
	}()
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
	pipe_connections(conn, remote_conn)
}

/*
Calls the appropriate handle_connections based on tunnel mode with enhanced features
*/
func handle_connection(conn net.Conn, tunnel bool) {
	source_ip := get_source_ip(conn)
	if tunnel {
		handle_tunnel_connection(conn)
	} else if address, err := handle_socks_connection(conn); err == nil {
		enhanced_server_response(conn, address, source_ip)
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
							return iface.Name + "\x00"
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
		lb_list[idx] = enhanced_load_balancer{address: fmt.Sprintf("%s:%d", lb_ip_or_fqdn, lb_port), iface: iface, contention_ratio: cont_ratio, current_connections: 0, source_ip_rules: make(map[string]source_ip_rule), source_ip_counters: make(map[string]int), total_connections: 0, success_count: 0, failure_count: 0, enabled: true}
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
	
	defer l.Close()
	defer stopWebServer()

	if (*quiet) {
		log.SetOutput(io.Discard)
	}

	mutex = &sync.Mutex{}
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
