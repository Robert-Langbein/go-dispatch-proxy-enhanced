// Network Topology Visualization - UniFi Style
class NetworkTopology {
    constructor() {
        this.svg = null;
        this.width = 0;
        this.height = 0;
        this.simulation = null;
        this.nodes = [];
        this.links = [];
        this.autoRefresh = false;
        this.refreshInterval = null;
        this.animationSpeed = 1;
        this.viewMode = 'traffic';
        this.zoomBehavior = null;
        this.transform = null;
        
        this.init();
    }

    init() {
        this.setupSVG();
        this.setupZoom();
        this.loadTopologyData();
        this.setupControls();
        
        // Auto-refresh every 5 seconds
        this.startAutoRefresh();
        
        // Handle window resize
        window.addEventListener('resize', () => this.handleResize());
    }

    setupSVG() {
        const container = document.getElementById('networkTopology');
        const rect = container.getBoundingClientRect();
        
        this.width = rect.width;
        this.height = rect.height;
        
        // Remove existing SVG
        d3.select(container).select('svg').remove();
        
        // Create new SVG
        this.svg = d3.select(container)
            .append('svg')
            .attr('width', this.width)
            .attr('height', this.height);
            
        // Create groups for different layers
        this.svg.append('defs');
        this.linkGroup = this.svg.append('g').attr('class', 'links');
        this.nodeGroup = this.svg.append('g').attr('class', 'nodes');
        this.flowGroup = this.svg.append('g').attr('class', 'flows');
        
        // Add gradient definitions for flows
        this.setupGradients();
    }

    setupGradients() {
        const defs = this.svg.select('defs');
        
        // High traffic gradient
        const highGradient = defs.append('linearGradient')
            .attr('id', 'high-traffic-gradient')
            .attr('gradientUnits', 'userSpaceOnUse');
        
        highGradient.append('stop')
            .attr('offset', '0%')
            .attr('stop-color', '#ff4757')
            .attr('stop-opacity', 0);
        
        highGradient.append('stop')
            .attr('offset', '50%')
            .attr('stop-color', '#ff4757')
            .attr('stop-opacity', 1);
            
        highGradient.append('stop')
            .attr('offset', '100%')
            .attr('stop-color', '#ff4757')
            .attr('stop-opacity', 0);
    }

    setupZoom() {
        this.zoomBehavior = d3.zoom()
            .scaleExtent([0.1, 4])
            .on('zoom', (event) => {
                this.transform = event.transform;
                this.nodeGroup.attr('transform', this.transform);
                this.linkGroup.attr('transform', this.transform);
                this.flowGroup.attr('transform', this.transform);
            });
            
        this.svg.call(this.zoomBehavior);
    }

    async loadTopologyData() {
        try {
            // Hide loading, show spinner
            document.querySelector('.topology-loading').classList.remove('hidden');
            
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
            
            // Transform data into nodes and links
            this.processTopologyData(config, stats);
            
            // Hide loading
            document.querySelector('.topology-loading').classList.add('hidden');
            
            // Render topology
            this.renderTopology();
            
        } catch (error) {
            console.error('Failed to load topology data:', error);
            this.showError('Failed to load network topology data');
        }
    }

    processTopologyData(config, stats) {
        this.nodes = [];
        this.links = [];
        
        // Add proxy node (central hub)
        const proxyNode = {
            id: 'proxy',
            type: 'proxy',
            name: 'Dispatch Proxy',
            ip: `${config.settings?.listen_host || '127.0.0.1'}:${config.settings?.listen_port || 8080}`,
            connections: stats.active_connections || 0,
            throughput: this.calculateTotalThroughput(stats),
            status: 'active',
            x: this.width / 2,
            y: this.height / 2,
            fx: this.width / 2, // Fixed position
            fy: this.height / 2
        };
        this.nodes.push(proxyNode);
        
        // Add load balancer nodes
        if (config.load_balancers && config.load_balancers.length > 0) {
            config.load_balancers.forEach((lb, index) => {
                const lbNode = {
                    id: `lb_${lb.address}`,
                    type: 'load-balancer',
                    name: `LB${lb.id || index + 1}`,
                    ip: lb.address,
                    interface: lb.interface,
                    ratio: lb.contention_ratio,
                    connections: lb.total_connections || 0,
                    throughput: this.formatBytes((lb.bytes_in || 0) + (lb.bytes_out || 0)),
                    status: lb.enabled ? 'active' : 'inactive',
                    enabled: lb.enabled
                };
                this.nodes.push(lbNode);
                
                // Create link between proxy and load balancer
                const traffic = this.calculateTrafficLevel(lb.total_connections || 0);
                this.links.push({
                    source: 'proxy',
                    target: lbNode.id,
                    type: 'proxy-to-lb',
                    traffic: traffic,
                    bandwidth: this.formatBytes((lb.bytes_in || 0) + (lb.bytes_out || 0)),
                    connections: lb.total_connections || 0,
                    enabled: lb.enabled
                });
            });
        }
        
        // Add sample client nodes (simulated based on connections)
        this.addSampleClients(stats);
        
        // Update statistics
        this.updateStatistics(config, stats);
    }

    addSampleClients(stats) {
        const clientCount = Math.min(Math.max(stats.active_connections || 0, 3), 8);
        const clients = [
            { name: 'MacBook Pro', icon: 'laptop', ip: '192.168.1.100' },
            { name: 'iPhone 13', icon: 'mobile-alt', ip: '192.168.1.101' },
            { name: 'iPad Air', icon: 'tablet-alt', ip: '192.168.1.102' },
            { name: 'Desktop PC', icon: 'desktop', ip: '192.168.1.103' },
            { name: 'Smart TV', icon: 'tv', ip: '192.168.1.104' },
            { name: 'Router', icon: 'wifi', ip: '192.168.1.1' },
            { name: 'Android Phone', icon: 'mobile-alt', ip: '192.168.1.105' },
            { name: 'Gaming Console', icon: 'gamepad', ip: '192.168.1.106' }
        ];
        
        for (let i = 0; i < clientCount; i++) {
            const client = clients[i] || { name: `Client ${i + 1}`, icon: 'laptop', ip: `192.168.1.${110 + i}` };
            const clientNode = {
                id: `client_${i}`,
                type: 'client',
                name: client.name,
                ip: client.ip,
                icon: client.icon,
                connections: Math.floor(Math.random() * 5) + 1,
                throughput: this.formatBytes(Math.random() * 1000000),
                status: 'active'
            };
            this.nodes.push(clientNode);
            
            // Create link between client and proxy
            const traffic = this.calculateTrafficLevel(clientNode.connections);
            this.links.push({
                source: clientNode.id,
                target: 'proxy',
                type: 'client-to-proxy',
                traffic: traffic,
                bandwidth: clientNode.throughput,
                connections: clientNode.connections,
                enabled: true
            });
        }
    }

    calculateTrafficLevel(connections) {
        if (connections >= 10) return 'high-traffic';
        if (connections >= 3) return 'medium-traffic';
        return 'low-traffic';
    }

    calculateTotalThroughput(stats) {
        return this.formatBytes((stats.bytes_in || 0) + (stats.bytes_out || 0));
    }

    formatBytes(bytes) {
        if (bytes === 0) return '0 B';
        const k = 1024;
        const sizes = ['B', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
    }

    renderTopology() {
        // Setup force simulation
        this.simulation = d3.forceSimulation(this.nodes)
            .force('link', d3.forceLink(this.links).id(d => d.id).distance(150))
            .force('charge', d3.forceManyBody().strength(-800))
            .force('center', d3.forceCenter(this.width / 2, this.height / 2))
            .force('collision', d3.forceCollide().radius(50));
        
        // Render links
        this.renderLinks();
        
        // Render nodes
        this.renderNodes();
        
        // Start flow animations
        this.animateFlows();
        
        // Update positions on simulation tick
        this.simulation.on('tick', () => {
            this.updatePositions();
        });
    }

    renderLinks() {
        const link = this.linkGroup
            .selectAll('.link')
            .data(this.links)
            .enter()
            .append('line')
            .attr('class', d => `link ${d.traffic}`)
            .attr('stroke-width', d => {
                switch(d.traffic) {
                    case 'high-traffic': return 4;
                    case 'medium-traffic': return 3;
                    default: return 2;
                }
            })
            .attr('stroke', d => {
                if (!d.enabled) return '#444';
                switch(d.traffic) {
                    case 'high-traffic': return '#ff4757';
                    case 'medium-traffic': return '#ffa502';
                    default: return '#2ed573';
                }
            })
            .attr('opacity', d => d.enabled ? 0.8 : 0.3);
    }

    renderNodes() {
        const node = this.nodeGroup
            .selectAll('.node')
            .data(this.nodes)
            .enter()
            .append('g')
            .attr('class', 'node')
            .call(d3.drag()
                .on('start', (event, d) => this.dragStarted(event, d))
                .on('drag', (event, d) => this.dragged(event, d))
                .on('end', (event, d) => this.dragEnded(event, d))
            )
            .on('click', (event, d) => this.showDeviceDetails(d));
        
        // Add circles for nodes
        node.append('circle')
            .attr('class', d => `node-circle ${d.type}`)
            .attr('r', d => {
                switch(d.type) {
                    case 'proxy': return 25;
                    case 'load-balancer': return 20;
                    default: return 15;
                }
            });
        
        // Add icons for nodes
        node.append('text')
            .attr('class', 'node-icon')
            .attr('text-anchor', 'middle')
            .attr('dy', '0.35em')
            .style('font-family', 'Font Awesome 6 Free')
            .style('font-weight', '900')
            .style('font-size', d => {
                switch(d.type) {
                    case 'proxy': return '16px';
                    case 'load-balancer': return '12px';
                    default: return '10px';
                }
            })
            .style('fill', 'white')
            .text(d => {
                switch(d.type) {
                    case 'proxy': return '\uf0e8'; // fa-sitemap
                    case 'load-balancer': return '\uf233'; // fa-server
                    default: return this.getClientIcon(d.icon);
                }
            });
        
        // Add labels
        node.append('text')
            .attr('class', 'node-label')
            .attr('text-anchor', 'middle')
            .attr('dy', '35px')
            .text(d => d.name);
        
        // Add stats
        node.append('text')
            .attr('class', 'node-stats')
            .attr('text-anchor', 'middle')
            .attr('dy', '48px')
            .text(d => d.throughput || d.ip);
    }

    getClientIcon(iconType) {
        const icons = {
            'laptop': '\uf109',     // fa-laptop
            'mobile-alt': '\uf3cd', // fa-mobile-alt
            'tablet-alt': '\uf3fa', // fa-tablet-alt
            'desktop': '\uf108',    // fa-desktop
            'tv': '\uf26c',         // fa-tv
            'wifi': '\uf1eb',       // fa-wifi
            'gamepad': '\uf11b'     // fa-gamepad
        };
        return icons[iconType] || '\uf109'; // default to laptop
    }

    animateFlows() {
        // Remove existing flow particles
        this.flowGroup.selectAll('.flow-particle').remove();
        
        // Add flow particles for active links
        this.links.filter(d => d.enabled).forEach(link => {
            this.createFlowParticles(link);
        });
    }

    createFlowParticles(link) {
        const particleCount = this.getParticleCount(link.traffic);
        
        for (let i = 0; i < particleCount; i++) {
            setTimeout(() => {
                const particle = this.flowGroup
                    .append('circle')
                    .attr('class', 'flow-particle')
                    .attr('r', 2)
                    .attr('fill', this.getParticleColor(link.traffic))
                    .attr('opacity', 0);
                
                this.animateParticle(particle, link);
            }, i * (2000 / particleCount) / this.animationSpeed);
        }
    }

    getParticleCount(traffic) {
        switch(traffic) {
            case 'high-traffic': return 6;
            case 'medium-traffic': return 4;
            default: return 2;
        }
    }

    getParticleColor(traffic) {
        switch(traffic) {
            case 'high-traffic': return '#ff4757';
            case 'medium-traffic': return '#ffa502';
            default: return '#2ed573';
        }
    }

    animateParticle(particle, link) {
        const sourceNode = this.nodes.find(n => n.id === link.source.id || n.id === link.source);
        const targetNode = this.nodes.find(n => n.id === link.target.id || n.id === link.target);
        
        if (!sourceNode || !targetNode) return;
        
        particle
            .attr('cx', sourceNode.x)
            .attr('cy', sourceNode.y)
            .transition()
            .duration(2000 / this.animationSpeed)
            .ease(d3.easeLinear)
            .attr('cx', targetNode.x)
            .attr('cy', targetNode.y)
            .attr('opacity', 1)
            .transition()
            .duration(200)
            .attr('opacity', 0)
            .remove();
    }

    updatePositions() {
        this.linkGroup.selectAll('.link')
            .attr('x1', d => d.source.x)
            .attr('y1', d => d.source.y)
            .attr('x2', d => d.target.x)
            .attr('y2', d => d.target.y);
        
        this.nodeGroup.selectAll('.node')
            .attr('transform', d => `translate(${d.x},${d.y})`);
    }

    // Event handlers
    dragStarted(event, d) {
        if (!event.active) this.simulation.alphaTarget(0.3).restart();
        d.fx = d.x;
        d.fy = d.y;
    }

    dragged(event, d) {
        d.fx = event.x;
        d.fy = event.y;
    }

    dragEnded(event, d) {
        if (!event.active) this.simulation.alphaTarget(0);
        if (d.id !== 'proxy') { // Keep proxy fixed
            d.fx = null;
            d.fy = null;
        }
    }

    showDeviceDetails(device) {
        const panel = document.getElementById('deviceDetailsPanel');
        const title = document.getElementById('deviceTitle');
        const content = document.getElementById('deviceContent');
        
        title.textContent = device.name;
        
        content.innerHTML = `
            <div style="margin-bottom: 16px;">
                <h4 style="margin: 0 0 8px 0; color: var(--unifi-text-primary);">Device Information</h4>
                <p><strong>Type:</strong> ${device.type.replace('-', ' ')}</p>
                <p><strong>IP Address:</strong> ${device.ip}</p>
                <p><strong>Status:</strong> <span style="color: ${device.status === 'active' ? '#2ed573' : '#ff4757'}">${device.status}</span></p>
            </div>
            
            <div style="margin-bottom: 16px;">
                <h4 style="margin: 0 0 8px 0; color: var(--unifi-text-primary);">Traffic Statistics</h4>
                <p><strong>Connections:</strong> ${device.connections || 0}</p>
                <p><strong>Throughput:</strong> ${device.throughput || '0 B'}</p>
                ${device.ratio ? `<p><strong>Load Ratio:</strong> ${device.ratio}</p>` : ''}
                ${device.interface ? `<p><strong>Interface:</strong> ${device.interface}</p>` : ''}
            </div>
            
            ${device.type === 'load-balancer' ? `
            <div style="margin-bottom: 16px;">
                <h4 style="margin: 0 0 8px 0; color: var(--unifi-text-primary);">Load Balancer Controls</h4>
                <button class="btn btn-secondary btn-sm" onclick="toggleLoadBalancer('${device.id}')" style="margin-right: 8px;">
                    ${device.enabled ? 'Disable' : 'Enable'}
                </button>
                <button class="btn btn-danger btn-sm" onclick="removeLoadBalancer('${device.ip}')">
                    Remove
                </button>
            </div>
            ` : ''}
        `;
        
        panel.classList.add('active');
    }

    updateStatistics(config, stats) {
        document.getElementById('totalThroughput').textContent = 
            this.formatBytes((stats.bytes_in || 0) + (stats.bytes_out || 0)) + '/s';
        document.getElementById('activeConnections').textContent = stats.active_connections || 0;
        document.getElementById('loadBalancers').textContent = config.load_balancers?.length || 0;
        document.getElementById('uniqueClients').textContent = this.nodes.filter(n => n.type === 'client').length;
    }

    showError(message) {
        const container = document.getElementById('networkTopology');
        container.innerHTML = `
            <div class="topology-error">
                <i class="fas fa-exclamation-triangle"></i>
                <h3>Error Loading Topology</h3>
                <p>${message}</p>
                <button class="btn btn-primary" onclick="networkTopology.loadTopologyData()">
                    <i class="fas fa-retry"></i>
                    Retry
                </button>
            </div>
        `;
    }

    setupControls() {
        // Animation speed control
        document.getElementById('animationSpeed').addEventListener('input', (e) => {
            this.setAnimationSpeed(parseFloat(e.target.value));
        });
    }

    setAnimationSpeed(speed) {
        this.animationSpeed = speed;
        document.getElementById('speedValue').textContent = speed + 'x';
    }

    handleResize() {
        const container = document.getElementById('networkTopology');
        const rect = container.getBoundingClientRect();
        
        this.width = rect.width;
        this.height = rect.height;
        
        this.svg
            .attr('width', this.width)
            .attr('height', this.height);
        
        if (this.simulation) {
            this.simulation
                .force('center', d3.forceCenter(this.width / 2, this.height / 2))
                .alpha(0.3)
                .restart();
        }
    }

    startAutoRefresh() {
        this.autoRefresh = true;
        this.refreshInterval = setInterval(() => {
            if (this.autoRefresh) {
                this.loadTopologyData();
                this.animateFlows(); // Refresh flow animations
            }
        }, 5000);
        
        document.getElementById('autoRefreshBtn').innerHTML = '<i class="fas fa-pause"></i>';
    }

    stopAutoRefresh() {
        this.autoRefresh = false;
        if (this.refreshInterval) {
            clearInterval(this.refreshInterval);
        }
        
        document.getElementById('autoRefreshBtn').innerHTML = '<i class="fas fa-play"></i>';
    }
}

// Global functions for HTML handlers
let networkTopology;

function refreshTopology() {
    networkTopology.loadTopologyData();
}

function toggleAutoRefresh() {
    if (networkTopology.autoRefresh) {
        networkTopology.stopAutoRefresh();
    } else {
        networkTopology.startAutoRefresh();
    }
}

function setAnimationSpeed(speed) {
    networkTopology.setAnimationSpeed(speed);
}

function changeViewMode(mode) {
    networkTopology.viewMode = mode;
    networkTopology.loadTopologyData();
}

function centerTopology() {
    if (networkTopology.svg && networkTopology.zoomBehavior) {
        networkTopology.svg
            .transition()
            .duration(750)
            .call(networkTopology.zoomBehavior.transform, d3.zoomIdentity);
    }
}

function closeDevicePanel() {
    document.getElementById('deviceDetailsPanel').classList.remove('active');
}

function toggleLoadBalancer(id) {
    // Implementation for toggling load balancer
    console.log('Toggle load balancer:', id);
}

function removeLoadBalancer(address) {
    // Implementation for removing load balancer
    console.log('Remove load balancer:', address);
}

// Initialize when DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
    networkTopology = new NetworkTopology();
}); 