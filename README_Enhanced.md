# Go Dispatch Proxy Enhanced

An enhanced SOCKS5 load balancing proxy with **source IP specific weighting** to combine multiple internet connections intelligently. Built on the original go-dispatch-proxy with advanced routing capabilities.

## 🚀 New Enhanced Features

### 1. **Source IP Specific Load Balancing**
- Configure different load balancing weights per source IP address
- Individual routing rules for specific clients or IP ranges
- JSON-based configuration with hot-reload support

### 2. **Enhanced Statistics & Monitoring**
- Real-time connection statistics per load balancer
- Success/failure rate tracking
- Source IP connection counters
- Detailed performance metrics

### 3. **Dynamic Load Balancer Management**
- Enable/disable load balancers at runtime
- Per-source-IP contention ratios
- Persistent configuration storage

## 📋 Enhanced Usage

### Basic Usage (backward compatible)
```bash
# Standard load balancing (works exactly like original)
./go-dispatch-proxy 192.168.1.10@3 10.81.201.18@2

# With enhanced logging
./go-dispatch-proxy -lhost 0.0.0.0 -lport 8080 192.168.1.10@3 10.81.201.18@2
```

### Enhanced Features

#### Show Load Balancer Statistics
```bash
./go-dispatch-proxy -stats 192.168.1.10@3 10.81.201.18@2
```

#### Custom Configuration File
```bash
./go-dispatch-proxy -config /path/to/custom_rules.json 192.168.1.10@3 10.81.201.18@2
```

#### Available Interfaces
```bash
./go-dispatch-proxy -list
```

## 🔧 Source IP Rules Configuration

Create a `source_ip_rules.json` file to define custom routing:

```json
{
  "192.168.1.10:0": {
    "192.168.0.100": {
      "source_ip": "192.168.0.100",
      "contention_ratio": 5,
      "description": "High priority client - 5x more bandwidth"
    },
    "192.168.0.101": {
      "source_ip": "192.168.0.101", 
      "contention_ratio": 1,
      "description": "Low priority client - reduced bandwidth"
    },
    "10.0.0.0/24": {
      "source_ip": "10.0.0.0/24",
      "contention_ratio": 3,
      "description": "VPN subnet - medium priority"
    }
  },
  "10.81.201.18:0": {
    "192.168.0.100": {
      "source_ip": "192.168.0.100",
      "contention_ratio": 2,
      "description": "Mobile connection routing"
    }
  }
}
```

### Configuration Structure

- **Top Level**: Load balancer address (e.g., `"192.168.1.10:0"`)
- **Second Level**: Source IP or CIDR range (e.g., `"192.168.0.100"` or `"10.0.0.0/24"`)
- **Rule Object**:
  - `source_ip`: The client IP or network range
  - `contention_ratio`: Custom weight for this source IP (higher = more connections)
  - `description`: Human-readable description

## 📊 Enhanced Logging

The enhanced proxy provides detailed logging:

```bash
[INFO] Enhanced SOCKS5 proxy with source IP load balancing started on 127.0.0.1:8080
[INFO] Load balancing 2 interfaces with enhanced features
[INFO] LB 1: 192.168.1.10:0 (eth0) - Default ratio: 3, Custom rules: 2, Status: enabled
[INFO] LB 2: 10.81.201.18:0 (wwan0) - Default ratio: 2, Custom rules: 1, Status: enabled
[DEBUG] Selected LB 0 (192.168.1.10:0) for source 192.168.0.100, effective ratio: 5
[DEBUG] 192.168.0.100 -> google.com:443 via 192.168.1.10:0 LB: 0, Source: 192.168.0.100
```

## 🔄 How Source IP Load Balancing Works

1. **Default Behavior**: Without rules, uses standard round-robin with global contention ratios
2. **Source IP Detection**: Automatically detects client source IP from connection
3. **Rule Lookup**: Checks if custom rules exist for the source IP
4. **Enhanced Routing**: Applies custom contention ratio if rule exists, otherwise uses default
5. **Statistics Tracking**: Monitors success/failure rates per load balancer and source IP

### Example Scenarios

#### High Priority Client
```json
"192.168.0.100": {
  "contention_ratio": 10,
  "description": "CEO laptop - maximum bandwidth"
}
```
→ This client gets 10 connections before switching to next load balancer

#### Bandwidth Limiting
```json
"192.168.0.200": {
  "contention_ratio": 1,
  "description": "Guest network - limited bandwidth"
}
```
→ This client gets only 1 connection per round-robin cycle

#### Network Segmentation
```json
"10.0.0.0/24": {
  "contention_ratio": 3,
  "description": "Marketing department subnet"
}
```
→ All IPs in this range get medium priority (3 connections per cycle)

## 🚀 Migration from Original

The enhanced version is **100% backward compatible**:

1. **Drop-in replacement**: Use same command line arguments
2. **Automatic enhancement**: Gets source IP awareness without configuration
3. **Optional features**: Add `source_ip_rules.json` when ready
4. **Performance**: Minimal overhead when not using custom rules

## 🔍 Monitoring & Troubleshooting

### Real-time Statistics
```bash
./go-dispatch-proxy -stats 192.168.1.10@3 10.81.201.18@2
```

Output:
```
--- Load Balancer Statistics ---
LB 1: 192.168.1.10:0 (eth0) - Status: enabled
  Default ratio: 3, Total connections: 1250
  Success: 1200, Failures: 50, Success rate: 96.00%
  Custom source IP rules (2):
    192.168.0.100 -> ratio: 5 (High priority client)
    192.168.0.101 -> ratio: 1 (Low priority client)
  Current source IP connections:
    192.168.0.100: 3
    192.168.0.101: 0

LB 2: 10.81.201.18:0 (wwan0) - Status: enabled
  Default ratio: 2, Total connections: 800
  Success: 780, Failures: 20, Success rate: 97.50%
  Custom source IP rules (0):
```

### Debug Logging
Add `-lhost 0.0.0.0` to see detailed connection routing information.

## 🔧 Advanced Configuration

### Environment Variables
- `GO_DISPATCH_CONFIG`: Override default config file path
- `GO_DISPATCH_DEBUG`: Enable debug mode

### Performance Tuning
- **High Traffic**: Increase contention ratios for better distribution
- **Stability**: Lower ratios for more frequent switching
- **Prioritization**: Use ratios 1-10 for different service levels

## 📈 Use Cases

1. **Corporate Networks**: Different bandwidth allocation per department
2. **ISP Load Balancing**: Customer-specific routing policies  
3. **Gaming Networks**: Priority routing for VIP customers
4. **Development**: Testing with different network conditions per client
5. **Security**: Isolated routing for different security zones

---

**Backward Compatibility**: This enhanced version works exactly like the original go-dispatch-proxy when no configuration file is present. 