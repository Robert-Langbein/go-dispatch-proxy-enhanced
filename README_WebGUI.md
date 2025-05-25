# Go Dispatch Proxy Enhanced - Web GUI Documentation

## 🌐 Web GUI Overview

The enhanced Go Dispatch Proxy now includes a full-featured **Web GUI** for monitoring and managing your load balancing configuration in real-time.

## 🚀 Quick Start

### Enable Web GUI
```bash
# Start with Web GUI on port 80 (default credentials: admin/admin)
./go-dispatch-proxy-enhanced -web 80 192.168.1.10@3 192.168.1.20@2

# Custom port and credentials
WEB_USERNAME=myuser WEB_PASSWORD=mypass ./go-dispatch-proxy-enhanced -web 8080 192.168.1.10@3 192.168.1.20@2

# With custom configuration file
./go-dispatch-proxy-enhanced -web 80 -config custom_rules.json 192.168.1.10@3 192.168.1.20@2
```

### Access Web Interface
1. Open browser to `http://your-server-ip:port`
2. Login with credentials (default: admin/admin)
3. Monitor and manage your load balancers

## 🔐 Authentication & Security

### Environment Variables
```bash
export WEB_USERNAME="your_username"
export WEB_PASSWORD="your_secure_password"
./go-dispatch-proxy-enhanced -web 80 ...
```

### Security Features
- **Session-based authentication** with 24-hour timeout
- **Cookie-based sessions** for seamless navigation
- **Automatic logout** for security
- **HTTPS ready** (place behind reverse proxy for TLS)

## 📊 Dashboard Features

### System Overview
- **Real-time statistics** with auto-refresh every 5 seconds
- **Load balancer count** and status indicators  
- **Total connections** processed
- **Overall success rate** percentage
- **System uptime** tracking

### Load Balancer Management
- **Visual status indicators** (enabled/disabled)
- **Toggle switches** for real-time enable/disable
- **Performance metrics** per load balancer:
  - Total connections processed
  - Success/failure counts and rates
  - Interface information
  - Default contention ratios

### Source IP Rules Management
- **View existing rules** per load balancer
- **Add new rules** with custom contention ratios
- **Remove rules** with confirmation
- **Real-time rule updates** saved to configuration file

### Active Monitoring
- **Active source IPs** table showing:
  - Current connections per source
  - Assigned load balancer
  - Effective contention ratio
  - Real-time connection tracking

## 🔧 API Endpoints

The Web GUI provides RESTful API endpoints for automation:

### Statistics API
```bash
GET /api/stats
# Returns complete dashboard data in JSON format
```

### Configuration API
```bash
GET /api/config
# Returns current load balancer configuration
```

### Rules Management API
```bash
# Add new source IP rule
POST /api/rules
Content-Type: application/json
{
  "lb_address": "192.168.1.10:0",
  "source_ip": "192.168.0.100",
  "contention_ratio": 5,
  "description": "High priority client"
}

# Remove source IP rule
DELETE /api/rules?lb_address=192.168.1.10:0&source_ip=192.168.0.100
```

### Load Balancer Control API
```bash
# Enable/disable load balancer
POST /api/lb/toggle
Content-Type: application/json
{
  "lb_address": "192.168.1.10:0",
  "enabled": true
}
```

## 🎨 User Interface

### Modern Design
- **Responsive layout** works on desktop, tablet, and mobile
- **Beautiful gradient backgrounds** and clean cards
- **Interactive toggle switches** for load balancer control
- **Modal dialogs** for rule management
- **Color-coded status indicators** (green=enabled, red=disabled)

### Real-time Updates
- **Auto-refresh** every 5 seconds (pauses during user interaction)
- **Live statistics** without page reload
- **Instant feedback** for configuration changes
- **Smart refresh** that preserves user context

### User Experience
- **One-click rule management** with intuitive modals
- **Confirmation dialogs** for destructive actions
- **Helpful tooltips** and descriptions
- **Mobile-friendly interface** for remote management

## 🔄 Integration Examples

### Docker Deployment
```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o go-dispatch-proxy-enhanced

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/go-dispatch-proxy-enhanced /usr/local/bin/
EXPOSE 8080 80
ENV WEB_USERNAME=admin
ENV WEB_PASSWORD=changeme123
ENTRYPOINT ["go-dispatch-proxy-enhanced"]
CMD ["-web", "80", "-lport", "8080", "192.168.1.10@3", "192.168.1.20@2"]
```

### Systemd Service
```ini
[Unit]
Description=Go Dispatch Proxy Enhanced with Web GUI
After=network.target

[Service]
Type=simple
User=proxy
ExecStart=/usr/local/bin/go-dispatch-proxy-enhanced -web 80 -lport 8080 192.168.1.10@3 192.168.1.20@2
Environment=WEB_USERNAME=admin
Environment=WEB_PASSWORD=secure123
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

### Reverse Proxy (Nginx)
```nginx
server {
    listen 443 ssl;
    server_name proxy.yourdomain.com;
    
    ssl_certificate /path/to/certificate.crt;
    ssl_certificate_key /path/to/private.key;
    
    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## ⚡ Performance & Monitoring

### Resource Usage
- **Minimal overhead**: Web GUI adds <5MB memory usage
- **Efficient updates**: Only active dashboard elements refresh
- **Background processing**: Web server runs in separate goroutine
- **Graceful shutdown**: Clean termination of web server

### Monitoring Capabilities
- **Real-time connection tracking** per source IP
- **Load balancer health monitoring** with failure rate tracking
- **Historical statistics** accumulation
- **Configuration change auditing** with timestamps

### Troubleshooting
```bash
# Debug mode with detailed logging
./go-dispatch-proxy-enhanced -web 8080 -lhost 0.0.0.0 192.168.1.10@3 192.168.1.20@2

# Check web server logs
curl -v http://localhost:8080/login

# API endpoint testing
curl -X GET http://localhost:8080/api/stats -H "Cookie: session=your_session"
```

## 🎯 Use Cases

### Network Operations Center (NOC)
- **Central dashboard** for monitoring multiple connections
- **Real-time alerts** for load balancer failures
- **Quick configuration changes** without server restart
- **Historical performance tracking**

### Enterprise Load Balancing
- **Department-specific rules** via source IP configuration
- **Priority customer routing** with higher contention ratios
- **Maintenance mode** toggling for individual load balancers
- **Audit trail** of configuration changes

### Development & Testing
- **Easy rule testing** with immediate visual feedback
- **Performance benchmarking** with real-time statistics
- **Configuration experimentation** without file editing
- **Mobile monitoring** during deployment

---

## 🌟 Key Benefits

✅ **Zero Downtime Configuration**: Change rules without restarting  
✅ **Visual Management**: No more command-line configuration files  
✅ **Real-time Monitoring**: See exactly what's happening  
✅ **Mobile Friendly**: Manage from anywhere  
✅ **API Integration**: Automate with external tools  
✅ **Secure Authentication**: Environment-based credentials  
✅ **Professional UI**: Modern, responsive design  

The Web GUI transforms the Go Dispatch Proxy Enhanced from a command-line tool into a **professional-grade network management solution** suitable for enterprise environments. 