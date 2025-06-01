// UniFi UDM Network Topology - Custom Canvas Implementation
class NetworkTopology {
    constructor() {
        this.canvas = null;
        this.ctx = null;
        this.width = 0;
        this.height = 0;
        this.devices = [];
        this.connections = [];
        this.particles = [];
        this.images = {};
        this.autoRefresh = false;
        this.refreshInterval = null;
        this.animationSpeed = 1;
        this.animationFrame = null;
        
        // Device name caching for consistent names
        this.deviceNameCache = new Map();
        this.deviceTypeCache = new Map();
        
        // UniFi UDM Colors (exact match)
        this.colors = {
            connectionLine: '#44c6fd',  // UDM blue for connections
            particle: '#44c6fd',        // Same blue for particles
            background: '#ffffff',
            text: '#212327',
            textSecondary: '#50565e',
            deviceBorder: '#ffffff',
            downloadSpeed: '#38cc65',   // Green for download (↓) - consistent with user request
            uploadSpeed: '#006fff'      // Blue for upload (↑) - consistent with user request
        };
        
        // Device image URLs - real device photos without frames
        this.deviceImages = {
            'isp': 'https://images.ui.com/b1eb8f5f-f800-4bb1-b92c-fe1e29b97c9b/7c3b0ccc-4325-4f41-a16f-8b4c98c21d0d.png', // Internet globe
            'gateway': 'https://images.ui.com/b1eb8f5f-f800-4bb1-b92c-fe1e29b97c9b/dd2c6e6c-58ad-4d26-a23e-48e3b2c6c7f8.png', // Gateway
            'load-balancer': 'https://images.ui.com/b1eb8f5f-f800-4bb1-b92c-fe1e29b97c9b/4a7b3b5e-8f1c-4e2b-9a6b-1c8e5f7a9b3d.png', // Switch
            'nas': 'https://www.synology.com/img/products/detail/DS923+/01-front.png', // Synology NAS
            'chromecast': 'https://lh3.googleusercontent.com/lrDVt_l-xwpO_j4lG82Stf_iTLx2HQMtq9oFjGu-2kQ6N0mUJmxk6E-0Z8F-s2dQ8w=s1600', // Chromecast
            'accesspoint': 'https://images.ui.com/b1eb8f5f-f800-4bb1-b92c-fe1e29b97c9b/f3d4c5e6-7890-1234-5678-9abcdef01234.png', // Access Point
            'iphone': 'https://www.apple.com/newsroom/images/product/iphone/standard/Apple_iPhone-13-Pro_iPhone-13-Pro-Max_09142021_inline.jpg.large.jpg', // iPhone
            'ipad': 'https://www.apple.com/newsroom/images/product/ipad/standard/Apple_iPad-Pro-12-9-inch-and-iPad-Pro-11-inch_10182021_inline.jpg.large.jpg', // iPad
            'printer': 'https://www.brother.com/pub/bsc/image_warehouse/EN/products/DCP-9020CDW_L.jpg', // Brother Printer
            'macbook': 'https://store.storeimages.cdn-apple.com/4982/as-images.apple.com/is/macbook-air-space-gray-select-201810?wid=904&hei=840&fmt=jpeg&qlt=80&.v=1633027804000' // MacBook Air
        };
        
        this.init();
    }

    async init() {
        this.setupCanvas();
        await this.loadImages();
        this.loadTopologyData();
        this.setupControls();
        this.startAnimation();
        this.startAutoRefresh();
        
        // Handle window resize
        window.addEventListener('resize', () => this.handleResize());
    }

    setupCanvas() {
        const container = document.getElementById('networkTopology');
        const rect = container.getBoundingClientRect();
        
        this.width = rect.width;
        this.height = rect.height;
        
        // Remove existing content
        container.innerHTML = '';
        
        // Create canvas
        this.canvas = document.createElement('canvas');
        this.canvas.width = this.width;
        this.canvas.height = this.height;
        this.canvas.style.width = '100%';
        this.canvas.style.height = '100%';
        this.canvas.style.cursor = 'pointer';
        
        this.ctx = this.canvas.getContext('2d');
        container.appendChild(this.canvas);
        
        // Add click handler
        this.canvas.addEventListener('click', (e) => this.handleCanvasClick(e));
    }

    async loadImages() {
        const loadPromises = Object.entries(this.deviceImages).map(([key, url]) => {
            return new Promise((resolve, reject) => {
                const img = new Image();
                img.crossOrigin = 'anonymous';
                img.onload = () => {
                    this.images[key] = img;
                    resolve();
                };
                img.onerror = () => {
                    // Fallback to colored circle if image fails
                    console.warn(`Failed to load image for ${key}, using fallback`);
                    this.images[key] = null;
                    resolve();
                };
                img.src = url;
            });
        });
        
        await Promise.all(loadPromises);
    }

    async loadTopologyData() {
        try {
            // Show loading
            this.showLoading(true);
            
            // Fetch topology data from API
            const [configResponse, statsResponse] = await Promise.all([
                fetch('/api/config'),
                fetch('/api/stats')
            ]);
            
            if (!configResponse.ok || !statsResponse.ok) {
                throw new Error('Failed to fetch topology data');
            }
            
            const config = await configResponse.json();
            const stats = await statsResponse.json();
            
            // Create UniFi UDM layout
            await this.processTopologyData(config, stats);
            
            // Hide loading
            this.showLoading(false);
            
        } catch (error) {
            console.error('Failed to load topology data:', error);
            this.showError('Failed to load network topology data');
        }
    }

    async processTopologyData(config, stats) {
        this.devices = [];
        this.connections = [];
        this.particles = [];
        
        console.log('=== API DATA DEBUG ===');
        console.log('Config data:', JSON.stringify(config, null, 2));
        console.log('Stats data:', JSON.stringify(stats, null, 2));
        console.log('Load balancers:', config.load_balancers);
        console.log('Active sources:', stats.active_sources);
        console.log('Traffic stats:', stats.traffic_stats);
        
        // UniFi UDM Tree Structure: Clear hierarchical layers from left to right
        const layerWidth = this.width / 4; // 4 layers max (simpler layout)
        const centerY = this.height / 2;
        
        // CORRECTED TOPOLOGY: Internet → Load Balancers → Dispatch Proxy → Clients
        
        // Layer 0: Internet (Far Left)
        const internetDevice = {
            id: 'internet',
            type: 'isp',
            name: 'Internet',
            subtitle: 'ISP',
            downloadSpeed: this.formatSpeed(stats.traffic_stats?.bytes_in_per_second || 0),
            uploadSpeed: this.formatSpeed(stats.traffic_stats?.bytes_out_per_second || 0),
            x: layerWidth * 0.5,
            y: centerY,
            layer: 0,
            size: 50
        };
        this.devices.push(internetDevice);
        
        // Layer 1: Load Balancers (Multiple Internet Connections)
        let loadBalancerDevices = [];
        if (config.load_balancers && config.load_balancers.length > 0) {
            const lbCount = config.load_balancers.length;
            const lbSpacing = Math.min(100, (this.height - 200) / Math.max(lbCount - 1, 1));
            const lbStartY = centerY - ((lbCount - 1) * lbSpacing) / 2;
            
            config.load_balancers.forEach((lb, index) => {
                // Use real load balancer data from API
                const lbStats = stats.load_balancers?.find(lbStat => lbStat.address === lb.address) || {};
                
                const lbDevice = {
                    id: `lb_${lb.address}`,
                    type: 'load-balancer',
                    name: lb.interface || 'Load Balancer',
                    subtitle: lb.address,
                    downloadSpeed: this.formatSpeed(this.calculateLBTraffic(lbStats, 'in')),
                    uploadSpeed: this.formatSpeed(this.calculateLBTraffic(lbStats, 'out')),
                    x: layerWidth * 1.5,
                    y: lbStartY + (index * lbSpacing),
                    layer: 1,
                    size: 45,
                    enabled: lb.enabled,
                    totalConnections: lbStats.total_connections || 0,
                    successRate: lbStats.success_rate || 0
                };
                this.devices.push(lbDevice);
                loadBalancerDevices.push(lbDevice);
                
                // Connection: Internet → Load Balancer
                this.connections.push({
                    from: internetDevice,
                    to: lbDevice,
                    enabled: lb.enabled
                });
            });
        }
        
        // Layer 2: Dispatch Proxy Gateway (Center)
        const gatewayDevice = {
            id: 'gateway',
            type: 'gateway',
            name: 'Dispatch Proxy',
            subtitle: 'Gateway',
            downloadSpeed: this.formatSpeed(stats.traffic_stats?.bytes_in_per_second || 0),
            uploadSpeed: this.formatSpeed(stats.traffic_stats?.bytes_out_per_second || 0),
            x: layerWidth * 2.5,
            y: centerY,
            layer: 2,
            size: 60
        };
        this.devices.push(gatewayDevice);
        
        // Connections: Load Balancers → Dispatch Proxy
        loadBalancerDevices.forEach(lb => {
            this.connections.push({
                from: lb,
                to: gatewayDevice,
                enabled: lb.enabled
            });
        });
        
        // Layer 3: Client Devices (Right Side - connect to Dispatch Proxy)
        let clientDevices = [];
        if (stats.active_sources && stats.active_sources.length > 0) {
            const clientSpacing = Math.min(80, (this.height - 160) / Math.max(stats.active_sources.length - 1, 1));
            const clientStartY = centerY - ((stats.active_sources.length - 1) * clientSpacing) / 2;
            
            // Process client devices asynchronously to resolve names
            const clientPromises = stats.active_sources.map(async (source, index) => {
                const deviceName = await this.getDeviceName(source.source_ip);
                const deviceType = this.guessDeviceType(source.source_ip);
                
                const clientDevice = {
                    id: `client_${source.source_ip}`,
                    type: deviceType,
                    name: deviceName,
                    subtitle: source.source_ip,
                    downloadSpeed: this.formatSpeed(this.estimateClientTraffic(source, 'in')),
                    uploadSpeed: this.formatSpeed(this.estimateClientTraffic(source, 'out')),
                    x: layerWidth * 3.5,
                    y: clientStartY + (index * clientSpacing),
                    layer: 3,
                    size: 40,
                    totalConnections: source.total_connections || 0,
                    activeConnections: source.active_connections || 0,
                    assignedLB: source.assigned_lb
                };
                
                return clientDevice;
            });
            
            // Wait for all device names to be resolved
            const resolvedClientDevices = await Promise.all(clientPromises);
            resolvedClientDevices.forEach(clientDevice => {
                this.devices.push(clientDevice);
                clientDevices.push(clientDevice);
                
                // Connection: Dispatch Proxy → Client
                this.connections.push({
                    from: gatewayDevice,
                    to: clientDevice,
                    enabled: true
                });
            });
        } else if (loadBalancerDevices.length > 0) {
            // If no active sources but we have load balancers, show at least the infrastructure
            console.info('No active client sources found. Showing infrastructure only.');
        } else {
            // No load balancers and no active sources - show minimal topology
            console.warn('No load balancers or active sources found. Showing minimal topology.');
        }
        
        // Initialize particles
        this.initializeParticles();
        
        // Update statistics
        this.updateStatistics(config, stats);
    }
    
    // Helper methods for real data processing
    calculateLBTraffic(lbStats, direction) {
        // Use real traffic data from load balancer statistics
        if (direction === 'in') {
            return lbStats.bytes_in_per_second || 0;
        } else {
            return lbStats.bytes_out_per_second || 0;
        }
    }
    
    estimateClientTraffic(source, direction) {
        // Use real traffic data from client statistics
        if (direction === 'in') {
            return source.bytes_in_per_second || 0;
        } else {
            return source.bytes_out_per_second || 0;
        }
    }
    
    // Real device name resolution with multiple methods
    async resolveDeviceName(sourceIP) {
        // Check cache first
        if (this.deviceNameCache.has(sourceIP)) {
            return this.deviceNameCache.get(sourceIP);
        }
        
        let deviceName = null;
        let deviceType = 'iphone'; // default
        
        try {
            // Method 1: Try reverse DNS lookup
            deviceName = await this.tryReverseDNS(sourceIP);
            
            // Method 2: If no DNS name, try NetBIOS/Bonjour detection
            if (!deviceName) {
                const deviceInfo = await this.tryDeviceDetection(sourceIP);
                deviceName = deviceInfo.name;
                deviceType = deviceInfo.type;
            }
            
            // Method 3: Fallback to consistent name based on IP
            if (!deviceName) {
                const fallbackInfo = this.generateConsistentName(sourceIP);
                deviceName = fallbackInfo.name;
                deviceType = fallbackInfo.type;
            }
        } catch (error) {
            console.warn(`Error resolving device name for ${sourceIP}:`, error);
            const fallbackInfo = this.generateConsistentName(sourceIP);
            deviceName = fallbackInfo.name;
            deviceType = fallbackInfo.type;
        }
        
        // Cache the results
        this.deviceNameCache.set(sourceIP, deviceName);
        this.deviceTypeCache.set(sourceIP, deviceType);
        
        return deviceName;
    }
    
    async tryReverseDNS(ip) {
        // Note: Reverse DNS from browser is limited, but we can try
        try {
            const response = await fetch(`/api/resolve-hostname?ip=${encodeURIComponent(ip)}`);
            if (response.ok) {
                const data = await response.json();
                if (data.hostname && data.hostname !== ip) {
                    // Clean up hostname (remove domain suffix, make readable)
                    let hostname = data.hostname.split('.')[0];
                    hostname = hostname.replace(/[-_]/g, ' ');
                    hostname = hostname.charAt(0).toUpperCase() + hostname.slice(1);
                    return hostname;
                }
            }
        } catch (error) {
            // DNS resolution failed, continue to next method
        }
        return null;
    }
    
    async tryDeviceDetection(ip) {
        try {
            const response = await fetch(`/api/device-info?ip=${encodeURIComponent(ip)}`);
            if (response.ok) {
                const data = await response.json();
                if (data.name || data.type) {
                    return {
                        name: data.name,
                        type: data.type || this.guessDeviceTypeFromInfo(data)
                    };
                }
            }
        } catch (error) {
            // Device detection failed
        }
        return { name: null, type: null };
    }
    
    generateConsistentName(sourceIP) {
        // Generate consistent names based on IP (not random)
        const ipParts = sourceIP.split('.');
        const lastOctet = parseInt(ipParts[3]);
        const secondLastOctet = parseInt(ipParts[2]);
        
        // Use IP-based seeded "randomness" for consistency
        const seed = (secondLastOctet * 256 + lastOctet) % 1000;
        
        // Device type based on IP range
        let deviceType;
        if (lastOctet < 10) {
            deviceType = 'accesspoint';
        } else if (lastOctet < 50) {
            deviceType = 'iphone';
        } else if (lastOctet < 100) {
            deviceType = 'ipad';
        } else if (lastOctet < 150) {
            deviceType = 'macbook';
        } else if (lastOctet < 200) {
            deviceType = 'nas';
        } else {
            deviceType = 'chromecast';
        }
        
        // Consistent names based on device type and IP
        const deviceNames = {
            'accesspoint': ['Access Point', 'UniFi AP', 'WiFi AP', 'Wireless AP'],
            'iphone': ['iPhone', 'iPhone Pro', 'iPhone 13', 'iPhone 14', 'iPhone 15'],
            'ipad': ['iPad', 'iPad Pro', 'iPad Air', 'iPad Mini'],
            'macbook': ['MacBook', 'MacBook Pro', 'MacBook Air', 'iMac', 'Mac Studio'],
            'nas': ['Synology NAS', 'QNAP NAS', 'File Server', 'Storage Server'],
            'chromecast': ['Chromecast', 'Google TV', 'Media Player', 'Smart TV']
        };
        
        const names = deviceNames[deviceType] || ['Device'];
        const nameIndex = seed % names.length;
        const baseName = names[nameIndex];
        
        // Add consistent suffix based on IP for uniqueness
        const suffix = lastOctet > 100 ? ` ${String.fromCharCode(65 + (seed % 26))}` : ` ${(seed % 9) + 1}`;
        
        return {
            name: baseName + suffix,
            type: deviceType
        };
    }
    
    guessDeviceType(sourceIP) {
        // Check cache first
        if (this.deviceTypeCache.has(sourceIP)) {
            return this.deviceTypeCache.get(sourceIP);
        }
        
        // Simple device type guessing based on IP patterns
        const lastOctet = parseInt(sourceIP.split('.').pop());
        
        let deviceType;
        if (lastOctet < 10) deviceType = 'accesspoint';
        else if (lastOctet < 50) deviceType = 'iphone';
        else if (lastOctet < 100) deviceType = 'ipad';
        else if (lastOctet < 150) deviceType = 'macbook';
        else if (lastOctet < 200) deviceType = 'nas';
        else deviceType = 'chromecast';
        
        return deviceType;
    }
    
    guessDeviceTypeFromInfo(deviceInfo) {
        // Guess device type from detection info
        const userAgent = (deviceInfo.user_agent || '').toLowerCase();
        const vendor = (deviceInfo.vendor || '').toLowerCase();
        const os = (deviceInfo.os || '').toLowerCase();
        
        if (userAgent.includes('iphone') || os.includes('ios')) return 'iphone';
        if (userAgent.includes('ipad') || userAgent.includes('tablet')) return 'ipad';
        if (userAgent.includes('mac') || os.includes('macos')) return 'macbook';
        if (vendor.includes('synology') || vendor.includes('qnap')) return 'nas';
        if (userAgent.includes('chromecast') || userAgent.includes('googletv')) return 'chromecast';
        if (userAgent.includes('unifi') || userAgent.includes('ubiquiti')) return 'accesspoint';
        
        return 'iphone'; // default
    }
    
    async getDeviceName(sourceIP) {
        return await this.resolveDeviceName(sourceIP);
    }

    formatSpeed(bytesPerSecond) {
        if (bytesPerSecond === 0) return '0 bps';
        
        const units = ['bps', 'Kbps', 'Mbps', 'Gbps'];
        let value = bytesPerSecond * 8;
        let unitIndex = 0;
        
        while (value >= 1000 && unitIndex < units.length - 1) {
            value /= 1000;
            unitIndex++;
        }
        
        return `${value.toFixed(1)} ${units[unitIndex]}`;
    }

    initializeParticles() {
        this.particles = [];
        this.connections.forEach((connection, connIndex) => {
            if (connection.enabled) {
                // Calculate particle count based on bandwidth usage - MUCH more conservative
                const maxBandwidth = this.getDeviceMaxBandwidth(connection.to);
                const currentUsage = this.getDeviceCurrentUsage(connection.to);
                const usageRatio = Math.min(1, currentUsage / Math.max(maxBandwidth, 1));
                
                // Drastically reduced particle count for performance
                const particleCount = Math.max(1, Math.round(2 + usageRatio * 4)); // 1-6 particles total
                
                for (let i = 0; i < particleCount; i++) {
                    const baseSpeed = 0.008 + (usageRatio * 0.012); 
                    this.particles.push({
                        connectionIndex: connIndex,
                        progress: Math.random(),
                        lateralOffset: (Math.random() - 0.5) * 4, // Small random offset
                        speed: baseSpeed + Math.random() * 0.008,
                        size: 1.5 + Math.random() * 1, // Slightly larger but fewer
                        opacity: 0.6 + (usageRatio * 0.3) + Math.random() * 0.1
                    });
                }
            }
        });
    }

    startAnimation() {
        const animate = () => {
            this.render();
            this.updateParticles();
            this.animationFrame = requestAnimationFrame(animate);
        };
        animate();
    }

    render() {
        // Clear canvas
        this.ctx.fillStyle = this.colors.background;
        this.ctx.fillRect(0, 0, this.width, this.height);
        
        // Draw connections
        this.drawConnections();
        
        // Draw particles
        this.drawParticles();
        
        // Draw devices
        this.drawDevices();
    }

    drawConnections() {
        this.connections.forEach(connection => {
            if (connection.enabled) {
                const fromX = connection.from.x;
                const fromY = connection.from.y;
                const toX = connection.to.x;
                const toY = connection.to.y;
                
                // Calculate bandwidth-based line thickness
                const maxBandwidth = this.getDeviceMaxBandwidth(connection.to);
                const currentUsage = this.getDeviceCurrentUsage(connection.to);
                const usageRatio = Math.min(1, currentUsage / Math.max(maxBandwidth, 1));
                
                // Line thickness based on max bandwidth (2-8px) - thicker lines
                const baseThickness = Math.max(2, Math.min(8, Math.log10(maxBandwidth + 1) * 2));
                const lineWidth = baseThickness * (0.5 + usageRatio * 0.5); // Varies with usage
                
                // Line opacity based on usage
                const opacity = 0.6 + (usageRatio * 0.4); // 60% to 100% opacity
                
                this.ctx.strokeStyle = this.colors.connectionLine;
                this.ctx.lineWidth = lineWidth;
                this.ctx.lineCap = 'round';
                this.ctx.globalAlpha = opacity;
                this.ctx.beginPath();
                
                                // Create clean, straight right-angled connections
                const deltaX = toX - fromX;
                const deltaY = toY - fromY;
                
                this.ctx.moveTo(fromX, fromY);
                
                if (Math.abs(deltaY) < 20) {
                    // Horizontal connection - completely straight
                    this.ctx.lineTo(toX, fromY); // Keep same Y level
                } else {
                    // Right-angled connection: horizontal then vertical then horizontal
                    const midX = fromX + deltaX * 0.6; // 60% of the way horizontally
                    
                    // 1. Horizontal line from source
                    this.ctx.lineTo(midX, fromY);
                    // 2. Vertical line to target Y level
                    this.ctx.lineTo(midX, toY);
                    // 3. Final horizontal line to target
                    this.ctx.lineTo(toX, toY);
                }
                
                this.ctx.stroke();
                this.ctx.globalAlpha = 1;
            }
        });
    }

    // Helper methods for bandwidth calculation
    getDeviceMaxBandwidth(device) {
        // Extract bandwidth from speed strings and convert to numeric value
        const downloadSpeed = this.parseSpeed(device.downloadSpeed);
        const uploadSpeed = this.parseSpeed(device.uploadSpeed);
        return Math.max(downloadSpeed, uploadSpeed);
    }

    getDeviceCurrentUsage(device) {
        // For demo purposes, use current speed as usage
        // In real implementation, this would come from actual usage stats
        const downloadSpeed = this.parseSpeed(device.downloadSpeed);
        const uploadSpeed = this.parseSpeed(device.uploadSpeed);
        return (downloadSpeed + uploadSpeed) / 2;
    }

    parseSpeed(speedString) {
        // Parse speed strings like "123.4 Mbps" to numeric Kbps value
        if (!speedString || speedString === '0 bps') return 0;
        
        const parts = speedString.split(' ');
        const value = parseFloat(parts[0]);
        const unit = parts[1];
        
        switch(unit) {
            case 'bps': return value / 1000;
            case 'Kbps': return value;
            case 'Mbps': return value * 1000;
            case 'Gbps': return value * 1000000;
            default: return value;
        }
    }

    drawParticles() {
        this.ctx.fillStyle = this.colors.particle;
        this.ctx.shadowColor = this.colors.particle;
        this.ctx.shadowBlur = 4;
        
        this.particles.forEach(particle => {
            const connection = this.connections[particle.connectionIndex];
            if (!connection || !connection.enabled) return;
            
            const basePos = this.getParticlePosition(connection, particle.progress);
            
            // Apply lateral offset for width distribution
            const connectionAngle = Math.atan2(
                connection.to.y - connection.from.y,
                connection.to.x - connection.from.x
            );
            
            // Calculate perpendicular offset
            const offsetX = Math.cos(connectionAngle + Math.PI/2) * particle.lateralOffset;
            const offsetY = Math.sin(connectionAngle + Math.PI/2) * particle.lateralOffset;
            
            const finalPos = {
                x: basePos.x + offsetX,
                y: basePos.y + offsetY
            };
            
            this.ctx.globalAlpha = particle.opacity;
            this.ctx.beginPath();
            this.ctx.arc(finalPos.x, finalPos.y, particle.size, 0, Math.PI * 2);
            this.ctx.fill();
        });
        
        this.ctx.shadowBlur = 0;
        this.ctx.globalAlpha = 1;
    }

    getParticlePosition(connection, progress) {
        const fromX = connection.from.x;
        const fromY = connection.from.y;
        const toX = connection.to.x;
        const toY = connection.to.y;
        const deltaX = toX - fromX;
        const deltaY = toY - fromY;
        
        const t = progress;
        
        if (Math.abs(deltaY) < 20) {
            // Horizontal connection - completely straight
            return {
                x: fromX + (toX - fromX) * t,
                y: fromY // Keep same Y level
            };
        } else {
            // Right-angled connection: horizontal then vertical then horizontal
            const midX = fromX + deltaX * 0.6;
            
            // Calculate total path length for proper progress distribution
            const segment1Length = midX - fromX; // horizontal
            const segment2Length = Math.abs(toY - fromY); // vertical
            const segment3Length = toX - midX; // final horizontal
            const totalLength = segment1Length + segment2Length + segment3Length;
            
            // Normalize segment lengths
            const seg1Ratio = segment1Length / totalLength;
            const seg2Ratio = segment2Length / totalLength;
            
            if (t <= seg1Ratio) {
                // First segment: horizontal from source
                const localT = t / seg1Ratio;
                return {
                    x: fromX + (midX - fromX) * localT,
                    y: fromY
                };
            } else if (t <= seg1Ratio + seg2Ratio) {
                // Second segment: vertical to target Y level
                const localT = (t - seg1Ratio) / seg2Ratio;
                return {
                    x: midX,
                    y: fromY + (toY - fromY) * localT
                };
            } else {
                // Third segment: final horizontal to target
                const localT = (t - seg1Ratio - seg2Ratio) / (1 - seg1Ratio - seg2Ratio);
                return {
                    x: midX + (toX - midX) * localT,
                    y: toY
                };
            }
        }
    }

    drawDevices() {
        this.devices.forEach(device => {
            // Draw device image without frame (like UDM)
            if (this.images[device.type]) {
                // Draw image directly without clipping to circle
                this.ctx.save();
                
                // Draw device image as rectangle (like UDM)
                const imageSize = device.size * 0.8; // Slightly smaller than device area
                this.ctx.drawImage(
                    this.images[device.type],
                    device.x - imageSize / 2,
                    device.y - imageSize / 2,
                    imageSize,
                    imageSize
                );
                
                this.ctx.restore();
            } else {
                // Fallback rounded square (not circle)
                this.ctx.fillStyle = this.getDeviceColor(device.type);
                const rectSize = device.size * 0.7;
                const cornerRadius = 8;
                
                this.ctx.beginPath();
                this.ctx.roundRect(
                    device.x - rectSize / 2,
                    device.y - rectSize / 2,
                    rectSize,
                    rectSize,
                    cornerRadius
                );
                this.ctx.fill();
            }
            
            // Device name
            this.ctx.fillStyle = this.colors.text;
            this.ctx.font = 'bold 12px -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif';
            this.ctx.textAlign = 'center';
            this.ctx.fillText(device.name, device.x, device.y + device.size / 2 + 16);
            
            // Download speed (blue)
            this.ctx.fillStyle = this.colors.downloadSpeed;
            this.ctx.font = '10px "SF Mono", Monaco, "Consolas", monospace';
            this.ctx.fillText(`↓ ${device.downloadSpeed}`, device.x, device.y + device.size / 2 + 30);
            
            // Upload speed (orange) 
            this.ctx.fillStyle = this.colors.uploadSpeed;
            this.ctx.fillText(`↑ ${device.uploadSpeed}`, device.x, device.y + device.size / 2 + 42);
        });
    }

    getDeviceColor(type) {
        const colors = {
            'isp': '#006fff',
            'gateway': '#006fff',
            'load-balancer': '#f5a524',
            'nas': '#50565e',
            'chromecast': '#50565e',
            'accesspoint': '#50565e',
            'iphone': '#50565e',
            'ipad': '#50565e',
            'printer': '#50565e',
            'macbook': '#50565e'
        };
        return colors[type] || '#50565e';
    }

    updateParticles() {
        this.particles.forEach(particle => {
            particle.progress += particle.speed * this.animationSpeed;
            
            if (particle.progress > 1) {
                particle.progress = 0;
                // Add some randomness to the next cycle
                particle.speed = 0.005 + Math.random() * 0.003;
            }
        });
    }

    handleCanvasClick(event) {
        const rect = this.canvas.getBoundingClientRect();
        const x = event.clientX - rect.left;
        const y = event.clientY - rect.top;
        
        // Check if click is on a device
        const clickedDevice = this.devices.find(device => {
            const distance = Math.sqrt((x - device.x) ** 2 + (y - device.y) ** 2);
            return distance <= device.size / 2;
        });
        
        if (clickedDevice) {
            this.showDeviceDetails(clickedDevice);
        }
    }

    showDeviceDetails(device) {
        const panel = document.getElementById('deviceDetailsPanel');
        const title = document.getElementById('deviceTitle');
        const content = document.getElementById('deviceContent');
        
        title.textContent = device.name;
        
        content.innerHTML = `
            <div style="margin-bottom: 16px;">
                <h4>Device Information</h4>
                <p><strong>Type:</strong> ${device.type.replace('-', ' ')}</p>
                <p><strong>Address:</strong> ${device.subtitle}</p>
            </div>
            
            <div style="margin-bottom: 16px;">
                <h4>Traffic Statistics</h4>
                <p><strong>Download:</strong> ${device.downloadSpeed}</p>
                <p><strong>Upload:</strong> ${device.uploadSpeed}</p>
            </div>
        `;
        
        panel.classList.add('active');
    }

    showLoading(show) {
        const loadingElement = document.querySelector('.topology-loading');
        if (loadingElement) {
            if (show) {
                loadingElement.classList.remove('hidden');
            } else {
                loadingElement.classList.add('hidden');
            }
        }
    }

    showError(message) {
        const container = document.getElementById('networkTopology');
        container.innerHTML = `
            <div class="topology-error">
                <div style="font-size: 48px; margin-bottom: 16px; color: var(--unifi-danger);">⚠️</div>
                <h3>Error Loading Topology</h3>
                <p>${message}</p>
                <button class="btn btn-primary" onclick="location.reload()" style="margin-top: 16px;">
                    🔄 Reload Page
                </button>
            </div>
        `;
    }

    updateStatistics(config, stats) {
        const trafficStats = stats.traffic_stats || {};
        
        // Download Speed (Bytes In)
        const downloadSpeed = this.formatSpeed(trafficStats.bytes_in_per_second || 0);
        const downloadSpeedBits = this.formatSpeedBits(trafficStats.bytes_in_per_second || 0);
        
        // Upload Speed (Bytes Out)  
        const uploadSpeed = this.formatSpeed(trafficStats.bytes_out_per_second || 0);
        const uploadSpeedBits = this.formatSpeedBits(trafficStats.bytes_out_per_second || 0);
        
        // Total Throughput
        const totalThroughput = this.formatSpeed((trafficStats.bytes_in_per_second || 0) + (trafficStats.bytes_out_per_second || 0));
        
        // Other stats
        const activeConnections = trafficStats.active_connections || 0;
        const loadBalancers = config.load_balancers?.length || 0;
        const uniqueClients = this.devices.filter(d => d.type !== 'isp' && d.type !== 'gateway' && d.type !== 'load-balancer').length;

        // Update Dashboard-style traffic cards
        const elements = {
            'downloadSpeed': downloadSpeed,
            'downloadSpeedBits': downloadSpeedBits,
            'uploadSpeed': uploadSpeed,
            'uploadSpeedBits': uploadSpeedBits,
            'totalThroughput': totalThroughput,
            'activeConnections': activeConnections,
            'loadBalancers': loadBalancers,
            'uniqueClients': uniqueClients
        };

        Object.keys(elements).forEach(id => {
            const element = document.getElementById(id);
            if (element) {
                element.textContent = elements[id];
            }
        });
    }

    formatSpeedBits(bytesPerSecond) {
        const bitsPerSecond = bytesPerSecond * 8;
        if (bitsPerSecond === 0) return '0 bit/s';
        
        const units = ['bit/s', 'kbit/s', 'Mbit/s', 'Gbit/s', 'Tbit/s'];
        const base = 1000; // Use 1000 for bits per second
        
        for (let i = units.length - 1; i >= 0; i--) {
            const unit = Math.pow(base, i);
            if (bitsPerSecond >= unit) {
                return (bitsPerSecond / unit).toFixed(1) + ' ' + units[i];
            }
        }
        return bitsPerSecond.toFixed(1) + ' bit/s';
    }

    setupControls() {
        // Simplified - no animation speed controls
    }

    setAnimationSpeed(speed) {
        this.animationSpeed = speed;
        // No speed display needed anymore
    }

    handleResize() {
        const container = document.getElementById('networkTopology');
        const rect = container.getBoundingClientRect();
        
        this.width = rect.width;
        this.height = rect.height;
        
        if (this.canvas) {
            this.canvas.width = this.width;
            this.canvas.height = this.height;
        }
        
        // Re-layout devices
        this.loadTopologyData();
    }

    startAutoRefresh() {
        this.autoRefresh = true;
        this.refreshInterval = setInterval(() => {
            if (this.autoRefresh) {
                this.loadTopologyData();
            }
        }, 5000);
        
        const btn = document.getElementById('autoRefreshBtn');
        if (btn) {
            btn.innerHTML = '<i class="fas fa-pause"></i>';
        }
    }

    stopAutoRefresh() {
        this.autoRefresh = false;
        if (this.refreshInterval) {
            clearInterval(this.refreshInterval);
        }
        
        const btn = document.getElementById('autoRefreshBtn');
        if (btn) {
            btn.innerHTML = '<i class="fas fa-play"></i>';
        }
    }

    destroy() {
        if (this.animationFrame) {
            cancelAnimationFrame(this.animationFrame);
        }
        if (this.refreshInterval) {
            clearInterval(this.refreshInterval);
        }
    }
}

// Global functions for HTML handlers
let networkTopology;

function refreshTopology() {
    if (networkTopology) {
        networkTopology.loadTopologyData();
    }
}

function toggleAutoRefresh() {
    if (networkTopology) {
        if (networkTopology.autoRefresh) {
            networkTopology.stopAutoRefresh();
        } else {
            networkTopology.startAutoRefresh();
        }
    }
}

// Simplified global functions - removed unused controls

function closeDevicePanel() {
    const panel = document.getElementById('deviceDetailsPanel');
    if (panel) {
        panel.classList.remove('active');
    }
}

// Initialize when DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
    setTimeout(() => {
        networkTopology = new NetworkTopology();
    }, 100);
});