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
	"strings"
	"sync"
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
	LoadBalancers    []LoadBalancerWebInfo `json:"load_balancers"`
	TotalConnections int                   `json:"total_connections"`
	TotalSuccess     int                   `json:"total_success"`
	TotalFailures    int                   `json:"total_failures"`
	OverallSuccessRate float64            `json:"overall_success_rate"`
	ActiveSources    []SourceIPInfo        `json:"active_sources"`
	SystemInfo       SystemInfo            `json:"system_info"`
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

var (
	webServer *WebServer
	startTime time.Time
)

func init() {
	startTime = time.Now()
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
Handle dashboard
*/
func (ws *WebServer) handleDashboard(w http.ResponseWriter, r *http.Request) {
	ws.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		
		data := ws.getDashboardData()
		tmpl, err := template.New("dashboard").Parse(getDashboardHTML())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		
		tmpl.Execute(w, data)
	})(w, r)
}

/*
Handle API stats endpoint
*/
func (ws *WebServer) handleAPIStats(w http.ResponseWriter, r *http.Request) {
	ws.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		
		data := ws.getDashboardData()
		json.NewEncoder(w).Encode(data)
	})(w, r)
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
Handle static files
*/
func (ws *WebServer) handleStatic(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/static/")
	
	switch path {
	case "style.css":
		w.Header().Set("Content-Type", "text/css")
		w.Write([]byte(getCSS()))
	case "script.js":
		w.Header().Set("Content-Type", "application/javascript")
		w.Write([]byte(getJavaScript()))
	default:
		http.NotFound(w, r)
	}
}

/*
Get dashboard data
*/
func (ws *WebServer) getDashboardData() DashboardData {
	mutex.Lock()
	defer mutex.Unlock()
	
	var data DashboardData
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
			SourceIPRules:    lb.source_ip_rules,
			ActiveSources:    lb.source_ip_counters,
		}
		
		totalConnections += lb.total_connections
		totalSuccess += lb.success_count
		totalFailures += lb.failure_count
		
		// Collect source IP information
		for sourceIP, count := range lb.source_ip_counters {
			if sourceInfo, exists := sourceMap[sourceIP]; exists {
				sourceInfo.ActiveConnections += count
			} else {
				effectiveRatio := get_effective_contention_ratio(&lb, sourceIP)
				sourceMap[sourceIP] = &SourceIPInfo{
					SourceIP:          sourceIP,
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
	
	if totalConnections > 0 {
		data.OverallSuccessRate = float64(totalSuccess) / float64(totalSuccess + totalFailures) * 100
	}
	
	// Convert source map to slice
	for _, sourceInfo := range sourceMap {
		data.ActiveSources = append(data.ActiveSources, *sourceInfo)
	}
	
	data.SystemInfo = SystemInfo{
		Version:       "Enhanced v2.0",
		Uptime:        time.Since(startTime).Round(time.Second).String(),
		ConfigFile:    config_file,
		ListenAddress: fmt.Sprintf(":%d", 8080), // This should be dynamic
		TotalLBs:      len(lb_list),
		StartTime:     startTime,
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
Start web server in background
*/
func startWebServer(port int) {
	if port <= 0 {
		return // Web server disabled
	}
	
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