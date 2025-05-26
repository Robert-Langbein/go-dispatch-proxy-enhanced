// web_server.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

type WebServer struct {
	server   *http.Server
	username string
	password string
	sessions map[string]time.Time
	sessionMutex sync.RWMutex
}

type DashboardData struct {
	LoadBalancers       []LoadBalancerWebInfo `json:"load_balancers"`
	TotalConnections    int                   `json:"total_connections"`
	TotalSuccess        int                   `json:"total_success"`
	TotalFailures       int                   `json:"total_failures"`
	OverallSuccessRate  float64              `json:"overall_success_rate"`
	ActiveSources       []SourceIPInfo        `json:"active_sources"`
	SystemInfo          SystemInfo            `json:"system_info"`
	ActiveConnections   []active_connection   `json:"active_connections"`
	ConnectionHistory   []active_connection   `json:"connection_history"`
	TrafficStats        GlobalTrafficStats    `json:"traffic_stats"`
	GatewayConfig       GatewayWebInfo        `json:"gateway_config"`
}

type LoadBalancerWebInfo struct {
	ID               int                        `json:"id"`
	Address          string                     `json:"address"`
	Interface        string                     `json:"interface"`
	DefaultRatio     int                        `json:"default_ratio"`
	Enabled          bool                       `json:"enabled"`
	TotalConnections int                        `json:"total_connections"`
	SuccessCount     int                        `json:"success_count"`
	FailureCount     int                        `json:"failure_count"`
	SuccessRate      float64                    `json:"success_rate"`
	SourceIPRules    map[string]source_ip_rule  `json:"source_ip_rules"`
	ActiveSources    map[string]int             `json:"active_sources"`
}

type SourceIPInfo struct {
	SourceIP         string `json:"source_ip"`
	TotalConnections int    `json:"total_connections"`
	ActiveConnections int   `json:"active_connections"`
	AssignedLB       string `json:"assigned_lb"`
	EffectiveRatio   int    `json:"effective_ratio"`
}

type SystemInfo struct {
	Version         string    `json:"version"`
	Uptime          string    `json:"uptime"`
	ConfigFile      string    `json:"config_file"`
	ListenAddress   string    `json:"listen_address"`
	TotalLBs        int       `json:"total_lbs"`
	StartTime       time.Time `json:"start_time"`
}

type GlobalTrafficStats struct {
	TotalBytesIn         int64   `json:"total_bytes_in"`
	TotalBytesOut        int64   `json:"total_bytes_out"`
	TotalDataTransferred int64   `json:"total_data_transferred"`
	BytesPerSecond       int64   `json:"bytes_per_second"`
	BytesInPerSecond     int64   `json:"bytes_in_per_second"`
	BytesOutPerSecond    int64   `json:"bytes_out_per_second"`
	ConnectionsPerMinute int64   `json:"connections_per_minute"`
	ConnectionsPerSecond int64   `json:"connections_per_second"`
	ActiveConnections    int     `json:"active_connections"`
	TotalConnections     int     `json:"total_connections"`
	UptimeSeconds        float64 `json:"uptime_seconds"`
}

type GatewayWebInfo struct {
	Enabled           bool     `json:"enabled"`
	GatewayIP         string   `json:"gateway_ip"`
	SubnetCIDR        string   `json:"subnet_cidr"`
	TransparentPort   int      `json:"transparent_port"`
	DNSPort           int      `json:"dns_port"`
	NATInterface      string   `json:"nat_interface"`
	AutoConfigure     bool     `json:"auto_configure"`
	DHCPRangeStart    string   `json:"dhcp_range_start"`
	DHCPRangeEnd      string   `json:"dhcp_range_end"`
	IptablesRules     []string `json:"iptables_rules"`
	Status            string   `json:"status"`
}

// Real-time traffic monitoring
type TrafficSample struct {
	timestamp    time.Time
	totalBytesIn int64
	totalBytesOut int64
}

var (
	trafficSamples       []TrafficSample
	trafficSamplesMutex  sync.RWMutex
	lastSampleTime       time.Time
	currentBytesInPerSecond  int64
	currentBytesOutPerSecond int64
)

var (
	webServer *WebServer
	startTime time.Time
	webServerPort int
)

// Global settings struct to store current configuration
type GlobalSettings struct {
	ListenHost      string
	ListenPort      int
	WebPort         int
	ConfigFile      string
	TunnelMode      bool
	DebugMode       bool
	QuietMode       bool
	GatewayMode     bool
	GatewayIP       string
	SubnetCIDR      string
	TransparentPort int
	DNSPort         int
	NATInterface    string
	AutoConfig      bool
	DHCPStart       string
	DHCPEnd         string
}

var currentSettings GlobalSettings

func init() {
	startTime = time.Now()
	
	// Initialize traffic monitoring
	trafficSamples = make([]TrafficSample, 0, 10) // Keep last 10 seconds
	lastSampleTime = time.Now()
	
	// Start traffic monitoring goroutine
	go trafficMonitor()
}

// Traffic monitoring function
func trafficMonitor() {
	ticker := time.NewTicker(500 * time.Millisecond) // 0.5 second updates
	defer ticker.Stop()
	
	for range ticker.C {
		updateTrafficSpeed()
	}
}

// Update current bytes per second based on recent samples
func updateTrafficSpeed() {
	trafficSamplesMutex.Lock()
	defer trafficSamplesMutex.Unlock()
	
	now := time.Now()
	currentTotalIn := atomic.LoadInt64(&global_bytes_in)
	currentTotalOut := atomic.LoadInt64(&global_bytes_out)
	
	// Add current sample
	trafficSamples = append(trafficSamples, TrafficSample{
		timestamp:     now,
		totalBytesIn:  currentTotalIn,
		totalBytesOut: currentTotalOut,
	})
	
	// Remove samples older than 5 seconds
	cutoff := now.Add(-5 * time.Second)
	for i := 0; i < len(trafficSamples); i++ {
		if trafficSamples[i].timestamp.After(cutoff) {
			trafficSamples = trafficSamples[i:]
			break
		}
	}
	
	// Calculate current speed based on last 2-5 seconds
	if len(trafficSamples) >= 2 {
		latest := trafficSamples[len(trafficSamples)-1]
		earliest := trafficSamples[0]
		
		// Calculate bytes per second over the sample period
		timeDiff := latest.timestamp.Sub(earliest.timestamp).Seconds()
		bytesInDiff := latest.totalBytesIn - earliest.totalBytesIn
		bytesOutDiff := latest.totalBytesOut - earliest.totalBytesOut
		
		if timeDiff > 0 {
			atomic.StoreInt64(&currentBytesInPerSecond, int64(float64(bytesInDiff)/timeDiff))
			atomic.StoreInt64(&currentBytesOutPerSecond, int64(float64(bytesOutDiff)/timeDiff))
		}
	}
}

// Get current real-time speeds
func getCurrentBytesInPerSecond() int64 {
	return atomic.LoadInt64(&currentBytesInPerSecond)
}

func getCurrentBytesOutPerSecond() int64 {
	return atomic.LoadInt64(&currentBytesOutPerSecond)
}

// Get combined speed for backwards compatibility
func getCurrentBytesPerSecond() int64 {
	return getCurrentBytesInPerSecond() + getCurrentBytesOutPerSecond()
}

/*
Create a new web server instance
*/
func NewWebServer(port int) *WebServer {
	username := os.Getenv("WEB_USERNAME")
	password := os.Getenv("WEB_PASSWORD")
	
	if username == "" {
		username = "admin"
	}
	if password == "" {
		password = "admin"
	}

	return &WebServer{
		username: username,
		password: password,
		sessions: make(map[string]time.Time),
		server: &http.Server{
			Addr:         fmt.Sprintf(":%d", port),
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
	}
}

/*
Start the web server
*/
func (ws *WebServer) Start() error {
	// Setup routes
	http.HandleFunc("/", ws.handleDashboard)
	http.HandleFunc("/login", ws.handleLogin)
	http.HandleFunc("/logout", ws.handleLogout)
	http.HandleFunc("/api/stats", ws.handleAPIStats)
	http.HandleFunc("/api/config", ws.handleAPIConfig)
	http.HandleFunc("/api/rules", ws.handleAPIRules)
	http.HandleFunc("/api/lb/toggle", ws.handleAPIToggleLB)
	http.HandleFunc("/api/connections", ws.handleAPIConnections)
	http.HandleFunc("/api/traffic", ws.handleAPITraffic)
	http.HandleFunc("/api/traffic/chart", ws.handleAPITrafficChart)
	http.HandleFunc("/api/connection/weight", ws.handleAPIConnectionWeight)
	http.HandleFunc("/api/gateway", ws.handleAPIGateway)
	http.HandleFunc("/api/gateway/toggle", ws.handleAPIGatewayToggle)
	http.HandleFunc("/api/gateway/config", ws.handleAPIGatewayConfig)
	http.HandleFunc("/api/settings", ws.handleAPISettings)
	http.HandleFunc("/api/interfaces", ws.handleAPIInterfaces)
	http.HandleFunc("/api/lb/add", ws.handleAPIAddLB)
	http.HandleFunc("/api/lb/remove", ws.handleAPIRemoveLB)
	// Removed - not needed with SQLite storage
	// http.HandleFunc("/api/export", ws.handleAPIExport)
	// http.HandleFunc("/api/import", ws.handleAPIImport)
	http.HandleFunc("/api/reset", ws.handleAPIReset)
	http.HandleFunc("/api/restart", ws.handleAPIRestart)
	http.HandleFunc("/settings", ws.handleSettings)
	http.HandleFunc("/network", ws.handleNetwork)
	http.HandleFunc("/static/", ws.handleStatic)
	
	log.Printf("[INFO] Web GUI started on http://0.0.0.0%s", ws.server.Addr)
	log.Printf("[INFO] Login credentials - Username: %s, Password: %s", ws.username, ws.password)
	
	return ws.server.ListenAndServe()
}

/*
Stop the web server
*/
func (ws *WebServer) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return ws.server.Shutdown(ctx)
}

/*
Authentication middleware
*/
func (ws *WebServer) requireAuth(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Check for session cookie
		cookie, err := r.Cookie("session")
		if err == nil {
			ws.sessionMutex.RLock()
			sessionTime, exists := ws.sessions[cookie.Value]
			ws.sessionMutex.RUnlock()
			
			if exists && time.Since(sessionTime) < 24*time.Hour {
				// Valid session, update timestamp
				ws.sessionMutex.Lock()
				ws.sessions[cookie.Value] = time.Now()
				ws.sessionMutex.Unlock()
				handler(w, r)
				return
			}
		}
		
		// No valid session, redirect to login
		http.Redirect(w, r, "/login", http.StatusFound)
	}
}

/*
Generate session ID
*/
func (ws *WebServer) generateSessionID() string {
	return fmt.Sprintf("session_%d_%d", time.Now().UnixNano(), len(ws.sessions))
}

/*
Handle login
*/
func (ws *WebServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		username := r.FormValue("username")
		password := r.FormValue("password")
		
		if username == ws.username && password == ws.password {
			// Create session
			sessionID := ws.generateSessionID()
			ws.sessionMutex.Lock()
			ws.sessions[sessionID] = time.Now()
			ws.sessionMutex.Unlock()
			
			// Set session cookie
			http.SetCookie(w, &http.Cookie{
				Name:    "session",
				Value:   sessionID,
				Path:    "/",
				Expires: time.Now().Add(24 * time.Hour),
			})
			
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}
		
		// Invalid credentials
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(getLoginHTML("Invalid credentials")))
		return
	}
	
	// Show login form
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(getLoginHTML("")))
}

/*
Handle logout
*/
func (ws *WebServer) handleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session")
	if err == nil {
		ws.sessionMutex.Lock()
		delete(ws.sessions, cookie.Value)
		ws.sessionMutex.Unlock()
	}
	
	// Clear session cookie
	http.SetCookie(w, &http.Cookie{
		Name:    "session",
		Value:   "",
		Path:    "/",
		Expires: time.Now().Add(-time.Hour),
	})
	
	http.Redirect(w, r, "/login", http.StatusFound)
}

/*
Handle dashboard page
*/
func (ws *WebServer) handleDashboard(w http.ResponseWriter, r *http.Request) {
	// Check authentication first
	sessionID, err := r.Cookie("session")
	if err != nil || sessionID == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	ws.sessionMutex.RLock()
	sessionTime, exists := ws.sessions[sessionID.Value]
	ws.sessionMutex.RUnlock()

	if !exists || time.Since(sessionTime) > 24*time.Hour {
		// Session expired or doesn't exist
		ws.sessionMutex.Lock()
		delete(ws.sessions, sessionID.Value)
		ws.sessionMutex.Unlock()
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	// Update session timestamp
	ws.sessionMutex.Lock()
	ws.sessions[sessionID.Value] = time.Now()
	ws.sessionMutex.Unlock()

	// Set content type
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	
	data := ws.getDashboardData()
	
	// Load main dashboard template
	dashboardContent := getDashboardHTML()
	
	// Create template with functions
	tmpl := template.New("dashboard").Funcs(getTemplateFunctions())
	
	// Parse main dashboard template
	if _, err := tmpl.Parse(dashboardContent); err != nil {
		log.Printf("[ERROR] Error parsing dashboard template: %v", err)
		http.Error(w, "Error parsing dashboard template", http.StatusInternalServerError)
		return
	}
	
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("[ERROR] Error executing template: %v", err)
		http.Error(w, "Error executing template", http.StatusInternalServerError)
		return
	}
}

/*
Check if user is authenticated (without redirect)
*/
func (ws *WebServer) isAuthenticated(r *http.Request) bool {
	sessionID, err := r.Cookie("session")
	if err != nil || sessionID == nil {
		return false
	}

	ws.sessionMutex.RLock()
	sessionTime, exists := ws.sessions[sessionID.Value]
	ws.sessionMutex.RUnlock()

	if !exists || time.Since(sessionTime) > 24*time.Hour {
		return false
	}

	return true
}

/*
Handle API stats endpoint
*/
func (ws *WebServer) handleAPIStats(w http.ResponseWriter, r *http.Request) {
	if !ws.isAuthenticated(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	data := ws.getDashboardData()
	json.NewEncoder(w).Encode(data)
}

/*
Handle API config endpoint
*/
func (ws *WebServer) handleAPIConfig(w http.ResponseWriter, r *http.Request) {
	ws.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		
		if r.Method == "GET" {
			// Return current configuration
			config := map[string]interface{}{
				"config_file": config_file,
				"load_balancers": ws.getLoadBalancersConfig(),
			}
			json.NewEncoder(w).Encode(config)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})(w, r)
}

/*
Handle API rules endpoint for managing source IP rules
*/
func (ws *WebServer) handleAPIRules(w http.ResponseWriter, r *http.Request) {
	ws.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		
		switch r.Method {
		case "POST":
			// Add new rule
			var rule struct {
				LBAddress       string `json:"lb_address"`
				SourceIP        string `json:"source_ip"`
				ContentionRatio int    `json:"contention_ratio"`
				Description     string `json:"description"`
			}
			
			if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			
			success := add_source_ip_rule(rule.LBAddress, rule.SourceIP, rule.ContentionRatio, rule.Description)
			json.NewEncoder(w).Encode(map[string]bool{"success": success})
			
		case "DELETE":
			// Remove rule
			lbAddress := r.URL.Query().Get("lb_address")
			sourceIP := r.URL.Query().Get("source_ip")
			
			if lbAddress == "" || sourceIP == "" {
				http.Error(w, "Missing parameters", http.StatusBadRequest)
				return
			}
			
			success := remove_source_ip_rule(lbAddress, sourceIP)
			json.NewEncoder(w).Encode(map[string]bool{"success": success})
			
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})(w, r)
}

/*
Handle API toggle load balancer endpoint
*/
func (ws *WebServer) handleAPIToggleLB(w http.ResponseWriter, r *http.Request) {
	ws.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		
		var req struct {
			LBAddress string `json:"lb_address"`
			Enabled   bool   `json:"enabled"`
		}
		
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		
		success := set_load_balancer_status(req.LBAddress, req.Enabled)
		json.NewEncoder(w).Encode(map[string]bool{"success": success})
	})(w, r)
}

/*
Handle API connections endpoint for real-time connection monitoring
*/
func (ws *WebServer) handleAPIConnections(w http.ResponseWriter, r *http.Request) {
	ws.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		
		if r.Method == "GET" {
			// Get query parameters for filtering
			sourceFilter := r.URL.Query().Get("source")
			destFilter := r.URL.Query().Get("destination")
			limitStr := r.URL.Query().Get("limit")
			
			limit := 100 // Default limit
			if limitStr != "" {
				if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
					limit = parsedLimit
					if limit > 500 { // Maximum limit for performance
						limit = 500
					}
				}
			}
			
			connections := get_active_connections(sourceFilter, destFilter, limit)
			
			response := map[string]interface{}{
				"active_connections": connections,
				"total_count":       len(connections),
				"limit":             limit,
			}
			
			json.NewEncoder(w).Encode(response)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})(w, r)
}

/*
Handle API traffic endpoint for real-time traffic statistics
*/
func (ws *WebServer) handleAPITraffic(w http.ResponseWriter, r *http.Request) {
	ws.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		
		if r.Method == "GET" {
			uptime := time.Since(global_start_time)
			
			traffic := GlobalTrafficStats{
				TotalBytesIn:         atomic.LoadInt64(&global_bytes_in),
				TotalBytesOut:        atomic.LoadInt64(&global_bytes_out),
				TotalDataTransferred: atomic.LoadInt64(&total_data_transferred),
				ActiveConnections:    len(active_connections),
				UptimeSeconds:        uptime.Seconds(),
			}
			
			// Calculate rates - use real-time speed
			traffic.BytesInPerSecond = getCurrentBytesInPerSecond()
			traffic.BytesOutPerSecond = getCurrentBytesOutPerSecond()
			traffic.BytesPerSecond = getCurrentBytesPerSecond()
			
			json.NewEncoder(w).Encode(traffic)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})(w, r)
}

/*
Handle API traffic chart endpoint for real-time chart data
*/
func (ws *WebServer) handleAPITrafficChart(w http.ResponseWriter, r *http.Request) {
	ws.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		
		if r.Method == "GET" {
			trafficSamplesMutex.RLock()
			defer trafficSamplesMutex.RUnlock()
			
			// Prepare chart data for last 30 seconds
			now := time.Now()
			cutoff := now.Add(-30 * time.Second)
			
			var chartData []map[string]interface{}
			
			for _, sample := range trafficSamples {
				if sample.timestamp.After(cutoff) {
					chartData = append(chartData, map[string]interface{}{
						"timestamp":     sample.timestamp.UnixMilli(),
						"bytes_in":      sample.totalBytesIn,
						"bytes_out":     sample.totalBytesOut,
						"bytes_in_speed":  getCurrentBytesInPerSecond(),
						"bytes_out_speed": getCurrentBytesOutPerSecond(),
					})
				}
			}
			
			response := map[string]interface{}{
				"chart_data":           chartData,
				"current_bytes_in":     getCurrentBytesInPerSecond(),
				"current_bytes_out":    getCurrentBytesOutPerSecond(),
				"update_interval_ms":   500,
			}
			
			json.NewEncoder(w).Encode(response)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})(w, r)
}

/*
Handle API connection weight endpoint for individual connection weight management
*/
func (ws *WebServer) handleAPIConnectionWeight(w http.ResponseWriter, r *http.Request) {
	ws.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		
		if r.Method == "POST" {
			var req struct {
				SourceIP        string `json:"source_ip"`
				LBAddress       string `json:"lb_address"`
				ContentionRatio int    `json:"contention_ratio"`
				Description     string `json:"description"`
			}
			
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			
			success := add_source_ip_rule(req.LBAddress, req.SourceIP, req.ContentionRatio, req.Description)
			response := map[string]interface{}{
				"success": success,
				"message": "Connection weight updated successfully",
			}
			
			json.NewEncoder(w).Encode(response)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})(w, r)
}

/*
Handle static files
*/
func (ws *WebServer) handleStatic(w http.ResponseWriter, r *http.Request) {
	relativePath := strings.TrimPrefix(r.URL.Path, "/static/")
	
	// Security check: prevent directory traversal
	if strings.Contains(relativePath, "..") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	
	// Handle individual files
	filePath := filepath.Join("web", "static", relativePath)
	
	// Set content type based on extension
	ext := filepath.Ext(filePath)
	switch ext {
	case ".css":
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case ".js":
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	case ".html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case ".gif":
		w.Header().Set("Content-Type", "image/gif")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	default:
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	}
	
	// Add cache headers for static files
	w.Header().Set("Cache-Control", "public, max-age=3600")
	
	content := loadStaticFile(filePath)
	if strings.HasPrefix(content, "/* Error loading") || strings.HasPrefix(content, "// Error loading") {
		log.Printf("[WARN] Static file not found: %s", filePath)
		http.NotFound(w, r)
		return
	}
	
	w.Write([]byte(content))
}

/*
Get dashboard data
*/
func (ws *WebServer) getDashboardData() DashboardData {
	var data DashboardData
	
	// Collect load balancer data with minimal locking
	func() {
		mutex.Lock()
		defer mutex.Unlock()
		
		data.LoadBalancers = make([]LoadBalancerWebInfo, len(lb_list))
		
		totalConnections := 0
		totalSuccess := 0
		totalFailures := 0
		sourceMap := make(map[string]*SourceIPInfo)
		
		for i, lb := range lb_list {
			successRate := 0.0
			total := lb.success_count + lb.failure_count
			if total > 0 {
				successRate = float64(lb.success_count) / float64(total) * 100
			}
			
			// Deep copy maps to avoid race conditions
			sourceIPRulesCopy := make(map[string]source_ip_rule)
			for k, v := range lb.source_ip_rules {
				sourceIPRulesCopy[k] = v
			}
			
			activeSourcesCopy := make(map[string]int)
			for k, v := range lb.source_ip_counters {
				activeSourcesCopy[k] = v
			}
			
			data.LoadBalancers[i] = LoadBalancerWebInfo{
				ID:               i + 1,
				Address:          lb.address,
				Interface:        lb.iface,
				DefaultRatio:     lb.contention_ratio,
				Enabled:          lb.enabled,
				TotalConnections: lb.total_connections,
				SuccessCount:     lb.success_count,
				FailureCount:     lb.failure_count,
				SuccessRate:      successRate,
				SourceIPRules:    sourceIPRulesCopy,
				ActiveSources:    activeSourcesCopy,
			}
			
			totalConnections += lb.total_connections
			totalSuccess += lb.success_count
			totalFailures += lb.failure_count
			
			// Collect source IP information
			for sourceIP, count := range activeSourcesCopy {
				if sourceInfo, exists := sourceMap[sourceIP]; exists {
					sourceInfo.ActiveConnections += count
					sourceInfo.TotalConnections += lb.total_connections
				} else {
					effectiveRatio := get_effective_contention_ratio(&lb, sourceIP)
					sourceMap[sourceIP] = &SourceIPInfo{
						SourceIP:          sourceIP,
						TotalConnections:  lb.total_connections,
						ActiveConnections: count,
						AssignedLB:        lb.address,
						EffectiveRatio:    effectiveRatio,
					}
				}
			}
		}
		
		data.TotalConnections = totalConnections
		data.TotalSuccess = totalSuccess
		data.TotalFailures = totalFailures
		
		if (totalSuccess + totalFailures) > 0 {
			data.OverallSuccessRate = float64(totalSuccess) / float64(totalSuccess + totalFailures) * 100
		}
		
		// Convert source map to slice
		for _, sourceInfo := range sourceMap {
			data.ActiveSources = append(data.ActiveSources, *sourceInfo)
		}
		
		data.SystemInfo = SystemInfo{
			Version:       "Enhanced v3.0 Real-time",
			Uptime:        time.Since(global_start_time).Round(time.Second).String(),
			ConfigFile:    config_file,
			ListenAddress: fmt.Sprintf(":%d", webServerPort),
			TotalLBs:      len(lb_list),
			StartTime:     global_start_time,
		}
	}()
	
	// Get connection data with separate locking
	data.ActiveConnections = get_active_connections("", "", 50)
	
	// Get connection history with separate locking
	func() {
		connection_mutex.RLock()
		defer connection_mutex.RUnlock()
		
		if len(connection_history) > 20 {
			// Create a copy to avoid race conditions
			copyLen := 20
			data.ConnectionHistory = make([]active_connection, copyLen)
			copy(data.ConnectionHistory, connection_history[len(connection_history)-copyLen:])
		} else if len(connection_history) > 0 {
			data.ConnectionHistory = make([]active_connection, len(connection_history))
			copy(data.ConnectionHistory, connection_history)
		}
	}()
	
	// Get safe connection count
	var activeConnectionCount int
	func() {
		connection_mutex.RLock()
		defer connection_mutex.RUnlock()
		activeConnectionCount = len(active_connections)
	}()
	
	// Traffic statistics (atomic operations are thread-safe)
	uptime := time.Since(global_start_time)
	data.TrafficStats = GlobalTrafficStats{
		TotalBytesIn:         atomic.LoadInt64(&global_bytes_in),
		TotalBytesOut:        atomic.LoadInt64(&global_bytes_out),
		TotalDataTransferred: atomic.LoadInt64(&total_data_transferred),
		ActiveConnections:    activeConnectionCount,
		TotalConnections:     data.TotalConnections,
		UptimeSeconds:        uptime.Seconds(),
	}
	
	// Calculate traffic rates
	if uptime.Seconds() > 0 {
		// Use real-time speed instead of average since start
		data.TrafficStats.BytesInPerSecond = getCurrentBytesInPerSecond()
		data.TrafficStats.BytesOutPerSecond = getCurrentBytesOutPerSecond()
		data.TrafficStats.BytesPerSecond = getCurrentBytesPerSecond()
		data.TrafficStats.ConnectionsPerMinute = int64(float64(data.TotalConnections) / uptime.Minutes())
		data.TrafficStats.ConnectionsPerSecond = int64(float64(data.TotalConnections) / uptime.Seconds())
	}
	
	return data
}

/*
Get load balancers configuration
*/
func (ws *WebServer) getLoadBalancersConfig() []map[string]interface{} {
	mutex.Lock()
	defer mutex.Unlock()
	
	config := make([]map[string]interface{}, len(lb_list))
	for i, lb := range lb_list {
		config[i] = map[string]interface{}{
			"id":               i + 1,
			"address":          lb.address,
			"interface":        lb.iface,
			"contention_ratio": lb.contention_ratio,
			"enabled":          lb.enabled,
			"source_ip_rules":  lb.source_ip_rules,
		}
	}
	return config
}

/*
Template helper functions
*/
func getTemplateFunctions() template.FuncMap {
	return template.FuncMap{
		"add": func(a, b int) int {
			return a + b
		},
		"percentage": func(part, total int) float64 {
			if total == 0 {
				return 0
			}
			return float64(part) / float64(total) * 100
		},
		"formatBytes": func(bytes int64) string {
			if bytes == 0 {
				return "0 B"
			}
			units := []string{"B", "KB", "MB", "GB", "TB"}
			base := float64(1024)
			
			for i := len(units) - 1; i >= 0; i-- {
				unit := float64(1)
				for j := 0; j < i; j++ {
					unit *= base
				}
				if float64(bytes) >= unit {
					return fmt.Sprintf("%.1f %s", float64(bytes)/unit, units[i])
				}
			}
			return fmt.Sprintf("%d B", bytes)
		},
		"duration": func(startTime time.Time) string {
			duration := time.Since(startTime)
			if duration < time.Minute {
				return fmt.Sprintf("%ds", int(duration.Seconds()))
			} else if duration < time.Hour {
				return fmt.Sprintf("%dm %ds", int(duration.Minutes()), int(duration.Seconds())%60)
			} else {
				return fmt.Sprintf("%dh %dm", int(duration.Hours()), int(duration.Minutes())%60)
			}
		},
		"len": func(slice interface{}) int {
			switch s := slice.(type) {
			case []active_connection:
				return len(s)
			case []SourceIPInfo:
				return len(s)
			case map[string]source_ip_rule:
				return len(s)
			default:
				return 0
			}
		},
	}
}

/*
Initialize global settings from command line flags
This should be called from main.go after flag parsing
*/
func InitializeSettings(lhost string, lport int, webPort int, configFile string, tunnel bool, debug bool, quiet bool, gatewayMode bool, gatewayIP string, subnetCIDR string, transparentPort int, dnsPort int, natInterface string, autoConfig bool, dhcpStart string, dhcpEnd string) {
	currentSettings = GlobalSettings{
		ListenHost:      lhost,
		ListenPort:      lport,
		WebPort:         webPort,
		ConfigFile:      configFile,
		TunnelMode:      tunnel,
		DebugMode:       debug,
		QuietMode:       quiet,
		GatewayMode:     gatewayMode,
		GatewayIP:       gatewayIP,
		SubnetCIDR:      subnetCIDR,
		TransparentPort: transparentPort,
		DNSPort:         dnsPort,
		NATInterface:    natInterface,
		AutoConfig:      autoConfig,
		DHCPStart:       dhcpStart,
		DHCPEnd:         dhcpEnd,
	}
}

/*
Start web server in background
*/
func startWebServer(port int) {
	if port <= 0 {
		return // Web server disabled
	}
	
	webServerPort = port // Store port for use in dashboard
	webServer = NewWebServer(port)
	
	go func() {
		if err := webServer.Start(); err != nil && err != http.ErrServerClosed {
			log.Printf("[ERROR] Web server failed to start: %v", err)
		}
	}()
}

/*
Database-first restart - all configuration comes from database
*/
func restartWithDatabaseConfig() error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot get executable path: %v", err)
	}
	
	// No command line arguments needed - everything comes from database
	args := []string{executable}
	
	log.Printf("[INFO] Restarting with database configuration: %s", executable)
	
	// Execute the new process without any flags
	if err := syscall.Exec(executable, args, os.Environ()); err != nil {
		return fmt.Errorf("failed to restart: %v", err)
	}
	
	return nil
}

/*
Stop web server
*/
func stopWebServer() {
	if webServer != nil {
		webServer.Stop()
	}
}

/*
Handle Gateway API endpoint
*/
func (ws *WebServer) handleAPIGateway(w http.ResponseWriter, r *http.Request) {
	if !ws.isAuthenticated(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	
	gatewayInfo := GatewayWebInfo{
		Enabled:         gateway_cfg.enabled,
		GatewayIP:       gateway_cfg.gateway_ip,
		SubnetCIDR:      gateway_cfg.subnet_cidr,
		TransparentPort: gateway_cfg.transparent_port,
		DNSPort:         gateway_cfg.dns_port,
		NATInterface:    gateway_cfg.nat_interface,
		AutoConfigure:   gateway_cfg.auto_configure,
		DHCPRangeStart:  gateway_cfg.dhcp_range_start,
		DHCPRangeEnd:    gateway_cfg.dhcp_range_end,
		IptablesRules:   gateway_cfg.iptables_rules,
		Status:          getGatewayStatus(),
	}
	
	json.NewEncoder(w).Encode(gatewayInfo)
}

/*
Handle Gateway Toggle API endpoint
*/
func (ws *WebServer) handleAPIGatewayToggle(w http.ResponseWriter, r *http.Request) {
	if !ws.isAuthenticated(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Enabled bool `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	if request.Enabled && !gateway_cfg.enabled {
		// Enable gateway mode
		if err := initialize_gateway_mode(); err != nil {
			response := map[string]interface{}{
				"success": false,
				"error":   fmt.Sprintf("Failed to enable gateway mode: %v", err),
			}
			json.NewEncoder(w).Encode(response)
			return
		}
		gateway_cfg.enabled = true
		log.Printf("[INFO] Gateway mode enabled via WebUI")
	} else if !request.Enabled && gateway_cfg.enabled {
		// Disable gateway mode
		if err := cleanup_gateway_mode(); err != nil {
			log.Printf("[WARN] Error during gateway cleanup: %v", err)
		}
		gateway_cfg.enabled = false
		log.Printf("[INFO] Gateway mode disabled via WebUI")
	}

	response := map[string]interface{}{
		"success": true,
		"enabled": gateway_cfg.enabled,
		"status":  getGatewayStatus(),
	}
	json.NewEncoder(w).Encode(response)
}

/*
Handle Gateway Configuration API endpoint
*/
func (ws *WebServer) handleAPIGatewayConfig(w http.ResponseWriter, r *http.Request) {
	if !ws.isAuthenticated(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method == "GET" {
		// Return current configuration
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(gateway_cfg)
		return
	}

	if r.Method == "POST" {
		// Update configuration
		var newConfig gateway_config
		if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Validate configuration
		if newConfig.gateway_ip == "" || newConfig.subnet_cidr == "" {
			http.Error(w, "Gateway IP and Subnet CIDR are required", http.StatusBadRequest)
			return
		}

		// Update configuration (but don't enable automatically)
		wasEnabled := gateway_cfg.enabled
		gateway_cfg.gateway_ip = newConfig.gateway_ip
		gateway_cfg.subnet_cidr = newConfig.subnet_cidr
		gateway_cfg.transparent_port = newConfig.transparent_port
		gateway_cfg.dns_port = newConfig.dns_port
		gateway_cfg.nat_interface = newConfig.nat_interface
		gateway_cfg.auto_configure = newConfig.auto_configure
		gateway_cfg.dhcp_range_start = newConfig.dhcp_range_start
		gateway_cfg.dhcp_range_end = newConfig.dhcp_range_end

		// If gateway was enabled, restart it with new configuration
		if wasEnabled {
			cleanup_gateway_mode()
			if err := initialize_gateway_mode(); err != nil {
				response := map[string]interface{}{
					"success": false,
					"error":   fmt.Sprintf("Failed to restart gateway with new config: %v", err),
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(response)
				return
			}
		}

		response := map[string]interface{}{
			"success": true,
			"message": "Gateway configuration updated successfully",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

/*
Get current gateway status
*/
func getGatewayStatus() string {
	if !gateway_cfg.enabled {
		return "disabled"
	}
	
	// Check if transparent proxy is running
	// This is a simple check - in a real implementation you might want more sophisticated monitoring
	if gateway_cfg.transparent_port > 0 {
		return "active"
	}
	
	return "error"
}

/*
Handle Settings page
*/
func (ws *WebServer) handleSettings(w http.ResponseWriter, r *http.Request) {
	// Check authentication first
	sessionID, err := r.Cookie("session")
	if err != nil || sessionID == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	ws.sessionMutex.RLock()
	sessionTime, exists := ws.sessions[sessionID.Value]
	ws.sessionMutex.RUnlock()

	if !exists || time.Since(sessionTime) > 24*time.Hour {
		// Session expired or doesn't exist
		ws.sessionMutex.Lock()
		delete(ws.sessions, sessionID.Value)
		ws.sessionMutex.Unlock()
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	// Update session timestamp
	ws.sessionMutex.Lock()
	ws.sessions[sessionID.Value] = time.Now()
	ws.sessionMutex.Unlock()

	// Set content type
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	
	// Create data structure for settings template
	data := struct {
		Settings     map[string]interface{} `json:"settings"`
		GatewayConfig GatewayWebInfo        `json:"gateway_config"`
	}{
		Settings: map[string]interface{}{
			"ListenHost":  currentSettings.ListenHost,
			"ListenPort":  currentSettings.ListenPort,
			"WebPort":     currentSettings.WebPort,
			"ConfigFile":  currentSettings.ConfigFile,
			"TunnelMode":  currentSettings.TunnelMode,
			"DebugMode":   currentSettings.DebugMode,
		},
		GatewayConfig: GatewayWebInfo{
			Enabled:         gateway_cfg.enabled,
			GatewayIP:       gateway_cfg.gateway_ip,
			SubnetCIDR:      gateway_cfg.subnet_cidr,
			TransparentPort: gateway_cfg.transparent_port,
			DNSPort:         gateway_cfg.dns_port,
			NATInterface:    gateway_cfg.nat_interface,
			AutoConfigure:   gateway_cfg.auto_configure,
			DHCPRangeStart:  gateway_cfg.dhcp_range_start,
			DHCPRangeEnd:    gateway_cfg.dhcp_range_end,
			IptablesRules:   gateway_cfg.iptables_rules,
			Status:          getGatewayStatus(),
		},
	}
	
	// Load settings template
	templatePath := filepath.Join("web", "templates", "settings.html")
	content, err := os.ReadFile(templatePath)
	if err != nil {
		log.Printf("[ERROR] Could not load settings template: %v", err)
		http.Error(w, "Settings template not found", http.StatusInternalServerError)
		return
	}
	
	// Create template with functions
	tmpl := template.New("settings").Funcs(getTemplateFunctions())
	
	// Parse settings template
	if _, err := tmpl.Parse(string(content)); err != nil {
		log.Printf("[ERROR] Error parsing settings template: %v", err)
		http.Error(w, "Error parsing settings template", http.StatusInternalServerError)
		return
	}
	
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("[ERROR] Error executing settings template: %v", err)
		http.Error(w, "Error executing settings template", http.StatusInternalServerError)
		return
	}
}

/*
Handle Settings API endpoint
*/
func (ws *WebServer) handleAPISettings(w http.ResponseWriter, r *http.Request) {
	if !ws.isAuthenticated(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	
	if r.Method == "GET" {
		// Return current settings from global settings
		settings := map[string]interface{}{
			"listen_host":      currentSettings.ListenHost,
			"listen_port":      currentSettings.ListenPort,
			"web_port":         currentSettings.WebPort,
			"tunnel_mode":      currentSettings.TunnelMode,
			"debug_mode":       currentSettings.DebugMode,
			"quiet_mode":       currentSettings.QuietMode,
			"gateway_mode":     currentSettings.GatewayMode,
			"gateway_ip":       currentSettings.GatewayIP,
			"subnet_cidr":      currentSettings.SubnetCIDR,
			"transparent_port": currentSettings.TransparentPort,
			"dns_port":         currentSettings.DNSPort,
			"nat_interface":    currentSettings.NATInterface,
			"auto_config":      currentSettings.AutoConfig,
			"dhcp_start":       currentSettings.DHCPStart,
			"dhcp_end":         currentSettings.DHCPEnd,
		}
		json.NewEncoder(w).Encode(settings)
	} else if r.Method == "POST" {
		// Update settings that can be changed at runtime
		var newSettings map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&newSettings); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		updated := []string{}
		requires_restart := []string{}

		// Debug mode can be changed at runtime
		if debugMode, ok := newSettings["debug_mode"].(bool); ok {
			currentSettings.DebugMode = debugMode
			debug_mode = debugMode
			updated = append(updated, "debug_mode")
		}

		// Gateway mode can be toggled at runtime
		if gatewayMode, ok := newSettings["gateway_mode"].(bool); ok {
			currentSettings.GatewayMode = gatewayMode
			gateway_cfg.enabled = gatewayMode
			updated = append(updated, "gateway_mode")
		}

		// Gateway configuration that can be updated
		if gatewayIP, ok := newSettings["gateway_ip"].(string); ok && gatewayIP != "" {
			currentSettings.GatewayIP = gatewayIP
			gateway_cfg.gateway_ip = gatewayIP
			updated = append(updated, "gateway_ip")
		}

		if subnetCIDR, ok := newSettings["subnet_cidr"].(string); ok && subnetCIDR != "" {
			currentSettings.SubnetCIDR = subnetCIDR
			gateway_cfg.subnet_cidr = subnetCIDR
			updated = append(updated, "subnet_cidr")
		}

		if transparentPort, ok := newSettings["transparent_port"].(float64); ok {
			currentSettings.TransparentPort = int(transparentPort)
			gateway_cfg.transparent_port = int(transparentPort)
			updated = append(updated, "transparent_port")
		}

		if dnsPort, ok := newSettings["dns_port"].(float64); ok {
			currentSettings.DNSPort = int(dnsPort)
			gateway_cfg.dns_port = int(dnsPort)
			updated = append(updated, "dns_port")
		}

		if natInterface, ok := newSettings["nat_interface"].(string); ok {
			currentSettings.NATInterface = natInterface
			gateway_cfg.nat_interface = natInterface
			updated = append(updated, "nat_interface")
		}

		if autoConfig, ok := newSettings["auto_configure"].(bool); ok {
			currentSettings.AutoConfig = autoConfig
			gateway_cfg.auto_configure = autoConfig
			updated = append(updated, "auto_configure")
		}

		if dhcpStart, ok := newSettings["dhcp_start"].(string); ok && dhcpStart != "" {
			currentSettings.DHCPStart = dhcpStart
			gateway_cfg.dhcp_range_start = dhcpStart
			updated = append(updated, "dhcp_start")
		}

		if dhcpEnd, ok := newSettings["dhcp_end"].(string); ok && dhcpEnd != "" {
			currentSettings.DHCPEnd = dhcpEnd
			gateway_cfg.dhcp_range_end = dhcpEnd
			updated = append(updated, "dhcp_end")
		}

		// Settings that require restart - but still save them to database
		if listenHost, ok := newSettings["listen_host"].(string); ok && listenHost != "" {
			currentSettings.ListenHost = listenHost
			requires_restart = append(requires_restart, "listen_host")
			updated = append(updated, "listen_host")
		}
		if listenPort, ok := newSettings["listen_port"].(float64); ok {
			currentSettings.ListenPort = int(listenPort)
			requires_restart = append(requires_restart, "listen_port")
			updated = append(updated, "listen_port")
		}
		if webPort, ok := newSettings["web_port"].(float64); ok {
			currentSettings.WebPort = int(webPort)
			requires_restart = append(requires_restart, "web_port")
			updated = append(updated, "web_port")
		}
		if tunnelMode, ok := newSettings["tunnel_mode"].(bool); ok {
			currentSettings.TunnelMode = tunnelMode
			requires_restart = append(requires_restart, "tunnel_mode")
			updated = append(updated, "tunnel_mode")
		}


		// Save all settings to database
		dbSettings := DBSettings{
			ListenHost:  currentSettings.ListenHost,
			ListenPort:  currentSettings.ListenPort,
			WebPort:     currentSettings.WebPort,
			ConfigFile:  "deprecated", // Keep for DB compatibility
			TunnelMode:  currentSettings.TunnelMode,
			DebugMode:   currentSettings.DebugMode,
			QuietMode:   currentSettings.QuietMode,
		}
		if err := saveSettings(dbSettings); err != nil {
			log.Printf("[ERROR] Failed to save settings to database: %v", err)
		}

		// Save gateway config to database
		dbGatewayConfig := DBGatewayConfig{
			Enabled:         currentSettings.GatewayMode,
			GatewayIP:       currentSettings.GatewayIP,
			SubnetCIDR:      currentSettings.SubnetCIDR,
			TransparentPort: currentSettings.TransparentPort,
			DNSPort:         currentSettings.DNSPort,
			NATInterface:    currentSettings.NATInterface,
			AutoConfigure:   currentSettings.AutoConfig,
			DHCPRangeStart:  currentSettings.DHCPStart,
			DHCPRangeEnd:    currentSettings.DHCPEnd,
		}
		if err := saveGatewayConfig(dbGatewayConfig); err != nil {
			log.Printf("[ERROR] Failed to save gateway config to database: %v", err)
		}

		// Create detailed message
		message := "Settings updated successfully."
		if len(updated) > 0 {
			message += fmt.Sprintf(" Updated: %s.", strings.Join(updated, ", "))
		}
		if len(requires_restart) > 0 {
			message += fmt.Sprintf(" Note: %s require service restart to take effect.", strings.Join(requires_restart, ", "))
		}

		response := map[string]interface{}{
			"success":         true,
			"message":         message,
			"updated":         updated,
			"requires_restart": requires_restart,
		}
		json.NewEncoder(w).Encode(response)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

/*
Handle Network Interfaces API endpoint
*/
func (ws *WebServer) handleAPIInterfaces(w http.ResponseWriter, r *http.Request) {
	if !ws.isAuthenticated(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	
	if r.Method == "GET" {
		// Get available network interfaces (all interfaces, filtering is done client-side)
		interfaces := []map[string]interface{}{}
		
		ifaces, err := net.Interfaces()
		if err != nil {
			http.Error(w, "Failed to get interfaces", http.StatusInternalServerError)
			return
		}

		for _, iface := range ifaces {
			// Skip only loopback interfaces, include all others (up/down, with/without IP)
			if (iface.Flags&net.FlagLoopback == net.FlagLoopback) {
				continue
			}
			
			addrs, _ := iface.Addrs()
			var ipAddr string
			var hasIPv4 bool
			
			// Get IPv4 address if available
			for _, addr := range addrs {
				if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					if ipnet.IP.To4() != nil {
						ipAddr = ipnet.IP.String()
						hasIPv4 = true
						break
					}
				}
			}
			
			interfaceInfo := map[string]interface{}{
				"name":     iface.Name,
				"ip":       ipAddr,
				"has_ip":   hasIPv4,
				"up":       (iface.Flags & net.FlagUp) != 0,
				"mtu":      iface.MTU,
				"flags":    iface.Flags.String(),
			}
			interfaces = append(interfaces, interfaceInfo)
		}
		
		json.NewEncoder(w).Encode(interfaces)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

/*
Handle Add Load Balancer API endpoint
*/
func (ws *WebServer) handleAPIAddLB(w http.ResponseWriter, r *http.Request) {
	if !ws.isAuthenticated(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Address         string `json:"address"`
		Interface       string `json:"interface"`
		ContentionRatio int    `json:"contention_ratio"`
		TunnelMode      bool   `json:"tunnel_mode"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Validate input
	if request.Address == "" {
		response := map[string]interface{}{
			"success": false,
			"error":   "Address is required",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	if request.ContentionRatio <= 0 {
		request.ContentionRatio = 1
	}

	// Check if load balancer already exists
	mutex.Lock()
	for _, lb := range lb_list {
		if lb.address == request.Address {
			mutex.Unlock()
			response := map[string]interface{}{
				"success": false,
				"error":   "Load balancer with this address already exists",
			}
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	// Save to database first
	dbLB := DBLoadBalancer{
		Address:         request.Address,
		Interface:       request.Interface,
		ContentionRatio: request.ContentionRatio,
		Enabled:         true,
	}
	
	if err := saveLoadBalancer(dbLB); err != nil {
		mutex.Unlock()
		response := map[string]interface{}{
			"success": false,
			"error":   fmt.Sprintf("Failed to save load balancer to database: %v", err),
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Add new load balancer to the list
	newLB := enhanced_load_balancer{
		address:             request.Address,
		iface:               request.Interface,
		contention_ratio:    request.ContentionRatio,
		current_connections: 0,
		source_ip_rules:     make(map[string]source_ip_rule),
		source_ip_counters:  make(map[string]int),
		total_connections:   0,
		success_count:       0,
		failure_count:       0,
		enabled:             true,
		bytes_transferred:   0,
		last_traffic_update: time.Now(),
	}

	lb_list = append(lb_list, newLB)
	mutex.Unlock()

	log.Printf("[INFO] Added load balancer via WebUI: %s (%s) - ratio: %d", 
		request.Address, request.Interface, request.ContentionRatio)

	response := map[string]interface{}{
		"success": true,
		"message": "Load balancer added successfully and is now active.",
	}
	json.NewEncoder(w).Encode(response)
}

/*
Handle Remove Load Balancer API endpoint
*/
func (ws *WebServer) handleAPIRemoveLB(w http.ResponseWriter, r *http.Request) {
	if !ws.isAuthenticated(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Address string `json:"address"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Find and remove load balancer
	mutex.Lock()
	defer mutex.Unlock()

	for i, lb := range lb_list {
		if lb.address == request.Address {
			// Remove from database first
			if err := deleteLoadBalancer(request.Address); err != nil {
				response := map[string]interface{}{
					"success": false,
					"error":   fmt.Sprintf("Failed to delete from database: %v", err),
				}
				json.NewEncoder(w).Encode(response)
				return
			}
			
			// Remove from slice
			lb_list = append(lb_list[:i], lb_list[i+1:]...)
			
			log.Printf("[INFO] Removed load balancer via WebUI: %s", request.Address)
			
			response := map[string]interface{}{
				"success": true,
				"message": "Load balancer removed successfully.",
			}
			json.NewEncoder(w).Encode(response)
			return
		}
	}

	// Load balancer not found
	response := map[string]interface{}{
		"success": false,
		"error":   "Load balancer not found",
	}
	json.NewEncoder(w).Encode(response)
}

/*
Handle Export Configuration API endpoint
*/
func (ws *WebServer) handleAPIExport(w http.ResponseWriter, r *http.Request) {
	if !ws.isAuthenticated(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Create comprehensive export configuration
	config := map[string]interface{}{
		"version":   "3.0-enhanced",
		"timestamp": time.Now().Format(time.RFC3339),
		"settings": map[string]interface{}{
			"listen_host":      currentSettings.ListenHost,
			"listen_port":      currentSettings.ListenPort,
			"web_port":         currentSettings.WebPort,
			"config_file":      currentSettings.ConfigFile,
			"tunnel_mode":      currentSettings.TunnelMode,
			"debug_mode":       currentSettings.DebugMode,
			"quiet_mode":       currentSettings.QuietMode,
		},
		"load_balancers": ws.getLoadBalancersConfig(),
		"gateway": map[string]interface{}{
			"enabled":           gateway_cfg.enabled,
			"gateway_ip":        gateway_cfg.gateway_ip,
			"subnet_cidr":       gateway_cfg.subnet_cidr,
			"transparent_port":  gateway_cfg.transparent_port,
			"dns_port":          gateway_cfg.dns_port,
			"nat_interface":     gateway_cfg.nat_interface,
			"auto_configure":    gateway_cfg.auto_configure,
			"dhcp_range_start":  gateway_cfg.dhcp_range_start,
			"dhcp_range_end":    gateway_cfg.dhcp_range_end,
		},
		"statistics": map[string]interface{}{
			"total_connections": ws.getTotalConnections(),
			"uptime_seconds":    time.Since(global_start_time).Seconds(),
			"export_time":       time.Now().Unix(),
		},
	}

	json.NewEncoder(w).Encode(config)
}

/*
Get total connections across all load balancers
*/
func (ws *WebServer) getTotalConnections() int {
	mutex.Lock()
	defer mutex.Unlock()
	
	total := 0
	for _, lb := range lb_list {
		total += lb.total_connections
	}
	return total
}

/*
Handle Import Configuration API endpoint
*/
func (ws *WebServer) handleAPIImport(w http.ResponseWriter, r *http.Request) {
	if !ws.isAuthenticated(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var config map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Note: Full import functionality would require complex validation and restart
	// For now, acknowledge the import
	response := map[string]interface{}{
		"success": true,
		"message": "Configuration import received. Service restart required to apply all changes.",
	}
	json.NewEncoder(w).Encode(response)
}

/*
Handle Reset to Defaults API endpoint
*/
func (ws *WebServer) handleAPIReset(w http.ResponseWriter, r *http.Request) {
	if !ws.isAuthenticated(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Note: Full reset would require stopping services and resetting configurations
	// For now, acknowledge the reset request
	response := map[string]interface{}{
		"success": true,
		"message": "Reset to defaults initiated. Service restart required.",
	}
	json.NewEncoder(w).Encode(response)
}

/*
Handle Restart API endpoint
*/
func (ws *WebServer) handleAPIRestart(w http.ResponseWriter, r *http.Request) {
	if !ws.isAuthenticated(r) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Send success response first
	response := map[string]interface{}{
		"success": true,
		"message": "Service restarting...",
	}
	json.NewEncoder(w).Encode(response)

	// Close the response connection
	if flusher, ok := w.(http.Flusher); ok {
		flusher.Flush()
	}

	log.Printf("[INFO] Service restart requested via WebUI")

	// Schedule restart in a separate goroutine to allow response to be sent
	go func() {
		time.Sleep(2 * time.Second) // Give time for response to be sent
		log.Printf("[INFO] Restarting service to apply new settings...")
		
		// Graceful shutdown of current server
		if webServer != nil {
			webServer.Stop()
		}
		
		// Restart with database configuration (no flags needed)
		if err := restartWithDatabaseConfig(); err != nil {
			log.Printf("[ERROR] Failed to restart: %v", err)
			os.Exit(1)
		}
	}()
}

/*
Handle network topology page
*/
func (ws *WebServer) handleNetwork(w http.ResponseWriter, r *http.Request) {
	// Check authentication first
	sessionID, err := r.Cookie("session")
	if err != nil || sessionID == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	ws.sessionMutex.RLock()
	sessionTime, exists := ws.sessions[sessionID.Value]
	ws.sessionMutex.RUnlock()

	if !exists || time.Since(sessionTime) > 24*time.Hour {
		// Session expired or doesn't exist
		ws.sessionMutex.Lock()
		delete(ws.sessions, sessionID.Value)
		ws.sessionMutex.Unlock()
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	// Update session timestamp
	ws.sessionMutex.Lock()
	ws.sessions[sessionID.Value] = time.Now()
	ws.sessionMutex.Unlock()

	// Set content type
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	
	// Serve network topology template
	http.ServeFile(w, r, "web/templates/network.html")
}