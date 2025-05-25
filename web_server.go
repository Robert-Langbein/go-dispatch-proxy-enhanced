// web_server.go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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
	TotalBytesIn        int64   `json:"total_bytes_in"`
	TotalBytesOut       int64   `json:"total_bytes_out"`
	TotalDataTransferred int64  `json:"total_data_transferred"`
	BytesPerSecond      int64   `json:"bytes_per_second"`
	ConnectionsPerMinute int64  `json:"connections_per_minute"`
	ActiveConnections   int     `json:"active_connections"`
	TotalConnections    int     `json:"total_connections"`
	UptimeSeconds       float64 `json:"uptime_seconds"`
}

// Real-time traffic monitoring
type TrafficSample struct {
	timestamp time.Time
	totalBytes int64
}

var (
	trafficSamples     []TrafficSample
	trafficSamplesMutex sync.RWMutex
	lastSampleTime     time.Time
	currentBytesPerSecond int64
)

var (
	webServer *WebServer
	startTime time.Time
	webServerPort int
)

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
	ticker := time.NewTicker(time.Second)
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
	currentTotal := atomic.LoadInt64(&total_data_transferred)
	
	// Add current sample
	trafficSamples = append(trafficSamples, TrafficSample{
		timestamp:  now,
		totalBytes: currentTotal,
	})
	
	// Remove samples older than 5 seconds
	cutoff := now.Add(-5 * time.Second)
	for i := 0; i < len(trafficSamples); i++ {
		if trafficSamples[i].timestamp.After(cutoff) {
			trafficSamples = trafficSamples[i:]
			break
		}
	}
	
	// Calculate current speed based on last 3-5 seconds
	if len(trafficSamples) >= 2 {
		latest := trafficSamples[len(trafficSamples)-1]
		earliest := trafficSamples[0]
		
		// Calculate bytes per second over the sample period
		timeDiff := latest.timestamp.Sub(earliest.timestamp).Seconds()
		bytesDiff := latest.totalBytes - earliest.totalBytes
		
		if timeDiff > 0 {
			atomic.StoreInt64(&currentBytesPerSecond, int64(float64(bytesDiff)/timeDiff))
		}
	}
}

// Get current real-time speed
func getCurrentBytesPerSecond() int64 {
	return atomic.LoadInt64(&currentBytesPerSecond)
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
	http.HandleFunc("/api/connection/weight", ws.handleAPIConnectionWeight)
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
			traffic.BytesPerSecond = getCurrentBytesPerSecond()
			
			json.NewEncoder(w).Encode(traffic)
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
		data.TrafficStats.BytesPerSecond = getCurrentBytesPerSecond()
		data.TrafficStats.ConnectionsPerMinute = int64(float64(data.TotalConnections) / uptime.Minutes())
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
Stop web server
*/
func stopWebServer() {
	if webServer != nil {
		webServer.Stop()
	}
} 