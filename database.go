// database.go
package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	_ "modernc.org/sqlite"
)

var db *sql.DB

// Database schema structures
type DBSettings struct {
	ID              int    `json:"id"`
	ListenHost      string `json:"listen_host"`
	ListenPort      int    `json:"listen_port"`
	WebPort         int    `json:"web_port"`
	ConfigFile      string `json:"config_file"`
	TunnelMode      bool   `json:"tunnel_mode"`
	DebugMode       bool   `json:"debug_mode"`
	QuietMode       bool   `json:"quiet_mode"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

type DBLoadBalancer struct {
	ID                int    `json:"id"`
	Address           string `json:"address"`
	Interface         string `json:"interface"`
	ContentionRatio   int    `json:"contention_ratio"`
	Enabled           bool   `json:"enabled"`
	TotalConnections  int    `json:"total_connections"`
	SuccessCount      int    `json:"success_count"`
	FailureCount      int    `json:"failure_count"`
	BytesTransferred  int64  `json:"bytes_transferred"`
	CreatedAt         string `json:"created_at"`
	UpdatedAt         string `json:"updated_at"`
}

type DBGatewayConfig struct {
	ID              int    `json:"id"`
	Enabled         bool   `json:"enabled"`
	GatewayIP       string `json:"gateway_ip"`
	SubnetCIDR      string `json:"subnet_cidr"`
	TransparentPort int    `json:"transparent_port"`
	DNSPort         int    `json:"dns_port"`
	NATInterface    string `json:"nat_interface"`
	AutoConfigure   bool   `json:"auto_configure"`
	DHCPRangeStart  string `json:"dhcp_range_start"`
	DHCPRangeEnd    string `json:"dhcp_range_end"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

type DBSourceIPRule struct {
	ID              int    `json:"id"`
	LoadBalancerID  int    `json:"load_balancer_id"`
	SourceIP        string `json:"source_ip"`
	ContentionRatio int    `json:"contention_ratio"`
	Description     string `json:"description"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

// Default configuration values
var defaultSettings = DBSettings{
	ListenHost:      "127.0.0.1",
	ListenPort:      8080,
	WebPort:         8090,
	ConfigFile:      "source_ip_rules.json",
	TunnelMode:      false,
	DebugMode:       false,
	QuietMode:       false,
}

var defaultGatewayConfig = DBGatewayConfig{
	Enabled:         false,
	GatewayIP:       "192.168.100.1",
	SubnetCIDR:      "192.168.100.0/24",
	TransparentPort: 8888,
	DNSPort:         5353,
	NATInterface:    "",
	AutoConfigure:   true,
	DHCPRangeStart:  "192.168.100.10",
	DHCPRangeEnd:    "192.168.100.100",
}

/*
Initialize database connection and create tables
*/
func initDatabase() error {
	// Create data directory if it doesn't exist
	dataDir := "data"
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %v", err)
	}

	// Open database connection
	dbPath := filepath.Join(dataDir, "dispatch-proxy.db")
	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %v", err)
	}

	// Test connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %v", err)
	}

	// Create tables
	if err := createTables(); err != nil {
		return fmt.Errorf("failed to create tables: %v", err)
	}

	// Initialize default data
	if err := initializeDefaultData(); err != nil {
		return fmt.Errorf("failed to initialize default data: %v", err)
	}

	log.Printf("[INFO] Database initialized successfully: %s", dbPath)
	return nil
}

/*
Create database tables
*/
func createTables() error {
	// Settings table
	settingsTable := `
	CREATE TABLE IF NOT EXISTS settings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		listen_host TEXT NOT NULL DEFAULT '127.0.0.1',
		listen_port INTEGER NOT NULL DEFAULT 8080,
		web_port INTEGER NOT NULL DEFAULT 8090,
		config_file TEXT NOT NULL DEFAULT 'source_ip_rules.json',
		tunnel_mode BOOLEAN NOT NULL DEFAULT 0,
		debug_mode BOOLEAN NOT NULL DEFAULT 0,
		quiet_mode BOOLEAN NOT NULL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	// Load balancers table
	loadBalancersTable := `
	CREATE TABLE IF NOT EXISTS load_balancers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		address TEXT NOT NULL UNIQUE,
		interface TEXT DEFAULT '',
		contention_ratio INTEGER NOT NULL DEFAULT 1,
		enabled BOOLEAN NOT NULL DEFAULT 1,
		total_connections INTEGER DEFAULT 0,
		success_count INTEGER DEFAULT 0,
		failure_count INTEGER DEFAULT 0,
		bytes_transferred INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	// Gateway configuration table
	gatewayConfigTable := `
	CREATE TABLE IF NOT EXISTS gateway_config (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		enabled BOOLEAN NOT NULL DEFAULT 0,
		gateway_ip TEXT NOT NULL DEFAULT '192.168.100.1',
		subnet_cidr TEXT NOT NULL DEFAULT '192.168.100.0/24',
		transparent_port INTEGER NOT NULL DEFAULT 8888,
		dns_port INTEGER NOT NULL DEFAULT 5353,
		nat_interface TEXT DEFAULT '',
		auto_configure BOOLEAN NOT NULL DEFAULT 1,
		dhcp_range_start TEXT NOT NULL DEFAULT '192.168.100.10',
		dhcp_range_end TEXT NOT NULL DEFAULT '192.168.100.100',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	// Source IP rules table
	sourceIPRulesTable := `
	CREATE TABLE IF NOT EXISTS source_ip_rules (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		load_balancer_id INTEGER NOT NULL,
		source_ip TEXT NOT NULL,
		contention_ratio INTEGER NOT NULL DEFAULT 1,
		description TEXT DEFAULT '',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (load_balancer_id) REFERENCES load_balancers(id) ON DELETE CASCADE,
		UNIQUE(load_balancer_id, source_ip)
	);`

	// Statistics table for historical data
	statisticsTable := `
	CREATE TABLE IF NOT EXISTS statistics (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		total_connections INTEGER DEFAULT 0,
		total_bytes_in INTEGER DEFAULT 0,
		total_bytes_out INTEGER DEFAULT 0,
		uptime_seconds REAL DEFAULT 0,
		snapshot_time DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	tables := []string{
		settingsTable,
		loadBalancersTable,
		gatewayConfigTable,
		sourceIPRulesTable,
		statisticsTable,
	}

	for _, table := range tables {
		if _, err := db.Exec(table); err != nil {
			return fmt.Errorf("failed to create table: %v", err)
		}
	}

	return nil
}

/*
Initialize default data if tables are empty
*/
func initializeDefaultData() error {
	// Check if settings exist
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM settings").Scan(&count)
	if err != nil {
		return err
	}

	// Insert default settings if none exist
	if count == 0 {
		if err := saveSettings(defaultSettings); err != nil {
			return fmt.Errorf("failed to save default settings: %v", err)
		}
		log.Printf("[INFO] Initialized default settings")
	}

	// Check if gateway config exists
	err = db.QueryRow("SELECT COUNT(*) FROM gateway_config").Scan(&count)
	if err != nil {
		return err
	}

	// Insert default gateway config if none exist
	if count == 0 {
		if err := saveGatewayConfig(defaultGatewayConfig); err != nil {
			return fmt.Errorf("failed to save default gateway config: %v", err)
		}
		log.Printf("[INFO] Initialized default gateway configuration")
	}

	return nil
}

/*
Initialize webPort from command line flag on first startup
*/
func initializeWebPortFromFlag(webPort int) error {
	// Check if this is the first startup by checking if web_port is still default
	settings, err := loadSettings()
	if err != nil {
		return err
	}

	// If webPort from flag is different from database default, update it
	if webPort != defaultSettings.WebPort && settings.WebPort == defaultSettings.WebPort {
		settings.WebPort = webPort
		if err := saveSettings(settings); err != nil {
			return fmt.Errorf("failed to save webPort from flag: %v", err)
		}
		log.Printf("[INFO] Saved initial webPort %d from command line to database", webPort)
	}

	return nil
}

/*
Load settings from database
*/
func loadSettings() (DBSettings, error) {
	var settings DBSettings
	query := `
		SELECT id, listen_host, listen_port, web_port, config_file, 
		       tunnel_mode, debug_mode, quiet_mode, created_at, updated_at
		FROM settings ORDER BY updated_at DESC LIMIT 1`

	err := db.QueryRow(query).Scan(
		&settings.ID, &settings.ListenHost, &settings.ListenPort,
		&settings.WebPort, &settings.ConfigFile, &settings.TunnelMode,
		&settings.DebugMode, &settings.QuietMode, &settings.CreatedAt,
		&settings.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		// Return default settings if none found
		return defaultSettings, nil
	}

	return settings, err
}

/*
Save settings to database
*/
func saveSettings(settings DBSettings) error {
	query := `
		INSERT OR REPLACE INTO settings 
		(id, listen_host, listen_port, web_port, config_file, tunnel_mode, debug_mode, quiet_mode, updated_at)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`

	_, err := db.Exec(query,
		settings.ListenHost, settings.ListenPort, settings.WebPort,
		settings.ConfigFile, settings.TunnelMode, settings.DebugMode,
		settings.QuietMode,
	)

	if err != nil {
		return fmt.Errorf("failed to save settings: %v", err)
	}

	log.Printf("[INFO] Settings saved to database")
	return nil
}

/*
Load gateway configuration from database
*/
func loadGatewayConfig() (DBGatewayConfig, error) {
	var config DBGatewayConfig
	query := `
		SELECT id, enabled, gateway_ip, subnet_cidr, transparent_port, dns_port,
		       nat_interface, auto_configure, dhcp_range_start, dhcp_range_end,
		       created_at, updated_at
		FROM gateway_config ORDER BY updated_at DESC LIMIT 1`

	err := db.QueryRow(query).Scan(
		&config.ID, &config.Enabled, &config.GatewayIP, &config.SubnetCIDR,
		&config.TransparentPort, &config.DNSPort, &config.NATInterface,
		&config.AutoConfigure, &config.DHCPRangeStart, &config.DHCPRangeEnd,
		&config.CreatedAt, &config.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		// Return default config if none found
		return defaultGatewayConfig, nil
	}

	return config, err
}

/*
Save gateway configuration to database
*/
func saveGatewayConfig(config DBGatewayConfig) error {
	query := `
		INSERT OR REPLACE INTO gateway_config 
		(id, enabled, gateway_ip, subnet_cidr, transparent_port, dns_port, 
		 nat_interface, auto_configure, dhcp_range_start, dhcp_range_end, updated_at)
		VALUES (1, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`

	_, err := db.Exec(query,
		config.Enabled, config.GatewayIP, config.SubnetCIDR,
		config.TransparentPort, config.DNSPort, config.NATInterface,
		config.AutoConfigure, config.DHCPRangeStart, config.DHCPRangeEnd,
	)

	if err != nil {
		return fmt.Errorf("failed to save gateway config: %v", err)
	}

	log.Printf("[INFO] Gateway configuration saved to database")
	return nil
}

/*
Load all load balancers from database
*/
func loadLoadBalancers() ([]DBLoadBalancer, error) {
	query := `
		SELECT id, address, interface, contention_ratio, enabled,
		       total_connections, success_count, failure_count, bytes_transferred,
		       created_at, updated_at
		FROM load_balancers ORDER BY created_at ASC`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var loadBalancers []DBLoadBalancer
	for rows.Next() {
		var lb DBLoadBalancer
		err := rows.Scan(
			&lb.ID, &lb.Address, &lb.Interface, &lb.ContentionRatio,
			&lb.Enabled, &lb.TotalConnections, &lb.SuccessCount,
			&lb.FailureCount, &lb.BytesTransferred, &lb.CreatedAt, &lb.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		loadBalancers = append(loadBalancers, lb)
	}

	return loadBalancers, nil
}

/*
Save load balancer to database
*/
func saveLoadBalancer(lb DBLoadBalancer) error {
	if lb.ID == 0 {
		// Insert new load balancer
		query := `
			INSERT INTO load_balancers 
			(address, interface, contention_ratio, enabled, total_connections,
			 success_count, failure_count, bytes_transferred)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

		result, err := db.Exec(query,
			lb.Address, lb.Interface, lb.ContentionRatio, lb.Enabled,
			lb.TotalConnections, lb.SuccessCount, lb.FailureCount, lb.BytesTransferred,
		)
		if err != nil {
			return fmt.Errorf("failed to insert load balancer: %v", err)
		}

		id, _ := result.LastInsertId()
		log.Printf("[INFO] Load balancer %s added to database (ID: %d)", lb.Address, id)
	} else {
		// Update existing load balancer
		query := `
			UPDATE load_balancers 
			SET address = ?, interface = ?, contention_ratio = ?, enabled = ?,
			    total_connections = ?, success_count = ?, failure_count = ?,
			    bytes_transferred = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?`

		_, err := db.Exec(query,
			lb.Address, lb.Interface, lb.ContentionRatio, lb.Enabled,
			lb.TotalConnections, lb.SuccessCount, lb.FailureCount,
			lb.BytesTransferred, lb.ID,
		)
		if err != nil {
			return fmt.Errorf("failed to update load balancer: %v", err)
		}

		log.Printf("[INFO] Load balancer %s updated in database", lb.Address)
	}

	return nil
}

/*
Delete load balancer from database
*/
func deleteLoadBalancer(address string) error {
	query := "DELETE FROM load_balancers WHERE address = ?"
	result, err := db.Exec(query, address)
	if err != nil {
		return fmt.Errorf("failed to delete load balancer: %v", err)
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("load balancer not found: %s", address)
	}

	log.Printf("[INFO] Load balancer %s deleted from database", address)
	return nil
}

/*
Close database connection
*/
func closeDatabase() {
	if db != nil {
		db.Close()
		log.Printf("[INFO] Database connection closed")
	}
}

/*
Update current settings from database
*/
func syncSettingsFromDatabase() error {
	dbSettings, err := loadSettings()
	if err != nil {
		return fmt.Errorf("failed to load settings from database: %v", err)
	}

	// Update global settings
	currentSettings = GlobalSettings{
		ListenHost:      dbSettings.ListenHost,
		ListenPort:      dbSettings.ListenPort,
		WebPort:         dbSettings.WebPort,
		ConfigFile:      dbSettings.ConfigFile,
		TunnelMode:      dbSettings.TunnelMode,
		DebugMode:       dbSettings.DebugMode,
		QuietMode:       dbSettings.QuietMode,
	}

	// Update runtime flags
	debug_mode = dbSettings.DebugMode
	quiet_mode = dbSettings.QuietMode

	log.Printf("[INFO] Settings synchronized from database")
	return nil
}

/*
Update current gateway config from database
*/
func syncGatewayConfigFromDatabase() error {
	dbGatewayConfig, err := loadGatewayConfig()
	if err != nil {
		return fmt.Errorf("failed to load gateway config from database: %v", err)
	}

	// Update global gateway config
	gateway_cfg.enabled = dbGatewayConfig.Enabled
	gateway_cfg.gateway_ip = dbGatewayConfig.GatewayIP
	gateway_cfg.subnet_cidr = dbGatewayConfig.SubnetCIDR
	gateway_cfg.transparent_port = dbGatewayConfig.TransparentPort
	gateway_cfg.dns_port = dbGatewayConfig.DNSPort
	gateway_cfg.nat_interface = dbGatewayConfig.NATInterface
	gateway_cfg.auto_configure = dbGatewayConfig.AutoConfigure
	gateway_cfg.dhcp_range_start = dbGatewayConfig.DHCPRangeStart
	gateway_cfg.dhcp_range_end = dbGatewayConfig.DHCPRangeEnd

	// Update current settings
	currentSettings.GatewayMode = dbGatewayConfig.Enabled
	currentSettings.GatewayIP = dbGatewayConfig.GatewayIP
	currentSettings.SubnetCIDR = dbGatewayConfig.SubnetCIDR
	currentSettings.TransparentPort = dbGatewayConfig.TransparentPort
	currentSettings.DNSPort = dbGatewayConfig.DNSPort
	currentSettings.NATInterface = dbGatewayConfig.NATInterface
	currentSettings.AutoConfig = dbGatewayConfig.AutoConfigure
	currentSettings.DHCPStart = dbGatewayConfig.DHCPRangeStart
	currentSettings.DHCPEnd = dbGatewayConfig.DHCPRangeEnd

	log.Printf("[INFO] Gateway configuration synchronized from database")
	return nil
}

/*
Save statistics snapshot to database
*/
func saveStatisticsSnapshot() error {
	query := `
		INSERT INTO statistics (total_connections, total_bytes_in, total_bytes_out, uptime_seconds)
		VALUES (?, ?, ?, ?)`

	totalConnections := getTotalConnectionsFromLB()
	uptime := time.Since(global_start_time).Seconds()

	_, err := db.Exec(query,
		totalConnections,
		atomic.LoadInt64(&global_bytes_in),
		atomic.LoadInt64(&global_bytes_out),
		uptime,
	)

	if err != nil {
		return fmt.Errorf("failed to save statistics: %v", err)
	}

	return nil
}

/*
Get total connections from load balancers
*/
func getTotalConnectionsFromLB() int {
	mutex.Lock()
	defer mutex.Unlock()

	total := 0
	for _, lb := range lb_list {
		total += lb.total_connections
	}
	return total
}

/*
Load load balancers from database and populate lb_list
*/
func loadLoadBalancersFromDatabase() error {
	dbLoadBalancers, err := loadLoadBalancers()
	if err != nil {
		return fmt.Errorf("failed to load load balancers: %v", err)
	}

	mutex.Lock()
	defer mutex.Unlock()

	// Clear existing load balancers
	lb_list = []enhanced_load_balancer{}

	// Convert database load balancers to internal format
	for _, dbLB := range dbLoadBalancers {
		lb := enhanced_load_balancer{
			address:             dbLB.Address,
			iface:               dbLB.Interface,
			contention_ratio:    dbLB.ContentionRatio,
			current_connections: 0,
			source_ip_rules:     make(map[string]source_ip_rule),
			source_ip_counters:  make(map[string]int),
			total_connections:   dbLB.TotalConnections,
			success_count:       dbLB.SuccessCount,
			failure_count:       dbLB.FailureCount,
			enabled:             dbLB.Enabled,
			bytes_transferred:   dbLB.BytesTransferred,
			last_traffic_update: time.Now(),
		}

		lb_list = append(lb_list, lb)
	}

	log.Printf("[INFO] Loaded %d load balancers from database", len(lb_list))
	return nil
}

/*
Sync current load balancers to database
*/
func syncLoadBalancersToDatabase() error {
	mutex.Lock()
	defer mutex.Unlock()

	for i, lb := range lb_list {
		dbLB := DBLoadBalancer{
			Address:           lb.address,
			Interface:         lb.iface,
			ContentionRatio:   lb.contention_ratio,
			Enabled:           lb.enabled,
			TotalConnections:  lb.total_connections,
			SuccessCount:      lb.success_count,
			FailureCount:      lb.failure_count,
			BytesTransferred:  lb.bytes_transferred,
		}

		if err := saveLoadBalancer(dbLB); err != nil {
			log.Printf("[WARN] Failed to sync load balancer %s to database: %v", lb.address, err)
		}

		// Update the ID back to lb_list if it was a new insert
		lb_list[i].total_connections = lb.total_connections
		lb_list[i].success_count = lb.success_count
		lb_list[i].failure_count = lb.failure_count
		lb_list[i].bytes_transferred = lb.bytes_transferred
	}

	return nil
}

/*
Initialize settings from database (replacement for the old InitializeSettings function)
*/
func InitializeSettingsFromDatabase() error {
	// Settings are already loaded in syncSettingsFromDatabase()
	// This function is kept for compatibility and future extensions
	log.Printf("[INFO] Settings initialized from database")
	return nil
} 