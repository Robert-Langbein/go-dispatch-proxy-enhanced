// Settings Page JavaScript
class SettingsManager {
    constructor() {
        this.currentSettings = {};
        this.availableInterfaces = [];
        this.activeLoadBalancers = [];
        this.init();
    }

    init() {
        // Load initial data
        this.loadSettings();
        this.scanInterfaces();
        this.loadLoadBalancers();
        
        // Hide settings actions initially
        this.hideSettingsActions();
        
        // Setup form change tracking
        setTimeout(() => {
            this.trackFormChanges();
        }, 1000);
        
        // Auto-refresh every 30 seconds
        setInterval(() => {
            this.refreshSettings();
        }, 30000);
    }

    // Settings Section Management
    showSettingsSection(sectionId, navElement) {
        // Hide all sections
        document.querySelectorAll('.settings-section').forEach(section => {
            section.classList.remove('active');
        });
        
        // Remove active class from all nav items
        document.querySelectorAll('.settings-nav-item').forEach(item => {
            item.classList.remove('active');
        });
        
        // Show selected section
        const section = document.getElementById(sectionId + '-settings');
        if (section) {
            section.classList.add('active');
            section.classList.add('fade-in');
        }
        
        // Add active class to clicked nav item
        navElement.classList.add('active');
        
        // Load section-specific data
        if (sectionId === 'interfaces') {
            this.scanInterfaces();
        } else if (sectionId === 'gateway') {
            this.loadGatewayConfig();
        }
    }

    // Load current settings from API
    async loadSettings() {
        try {
            const response = await fetch('/api/settings');
            if (response.ok) {
                this.currentSettings = await response.json();
                this.populateSettingsForm();
            }
        } catch (error) {
            console.error('Failed to load settings:', error);
            this.showNotification('Failed to load settings', 'error');
        }
    }

    // Populate form with current settings
    populateSettingsForm() {
        // Populate based on available data or defaults
        const elements = {
            'lhost': this.currentSettings.listen_host || '127.0.0.1',
            'lport': this.currentSettings.listen_port || 8080,
            'webPort': this.currentSettings.web_port || 0,
            'tunnel': this.currentSettings.tunnel_mode || false,
            'debug': this.currentSettings.debug_mode || false
        };

        Object.entries(elements).forEach(([id, value]) => {
            const element = document.getElementById(id);
            if (element) {
                if (element.type === 'checkbox') {
                    element.checked = value;
                } else {
                    element.value = value;
                }
            }
        });
    }

    // Scan available network interfaces
    async scanInterfaces() {
        const container = document.getElementById('availableInterfaces');
        if (!container) return;
        
        container.innerHTML = '<div class="loading"><i class="fas fa-spinner fa-spin"></i>Scanning network interfaces...</div>';
        
        try {
            const response = await fetch('/api/interfaces');
            if (response.ok) {
                this.availableInterfaces = await response.json();
                this.renderInterfaces();
            } else {
                throw new Error('Failed to scan interfaces');
            }
        } catch (error) {
            console.error('Failed to scan interfaces:', error);
            container.innerHTML = '<div class="error-message"><i class="fas fa-exclamation-triangle"></i>Failed to scan network interfaces</div>';
        }
    }

    // Render available interfaces
    renderInterfaces() {
        const container = document.getElementById('availableInterfaces');
        if (!container || !this.availableInterfaces.length) {
            container.innerHTML = '<div class="error-message"><i class="fas fa-info-circle"></i>No network interfaces found</div>';
            return;
        }

        const interfacesHTML = this.availableInterfaces.map(iface => `
            <div class="interface-card" data-interface="${iface.name}">
                <div class="interface-card-header">
                    <div class="interface-name">
                        <i class="fas fa-network-wired"></i>
                        ${iface.name}
                    </div>
                    <div class="status-indicator ${iface.up ? '' : 'inactive'}"></div>
                </div>
                <div class="interface-ip">${iface.ip || 'No IP assigned'}</div>
                <div class="interface-status">
                    <span>${iface.up ? 'Active' : 'Inactive'}</span>
                    ${iface.speed ? `• ${iface.speed}` : ''}
                </div>
                <div style="margin-top: 12px;">
                    <button class="btn btn-primary btn-sm" onclick="settingsManager.addInterfaceAsLB('${iface.name}', '${iface.ip}')">
                        <i class="fas fa-plus"></i>
                        Add as Load Balancer
                    </button>
                </div>
            </div>
        `).join('');

        container.innerHTML = interfacesHTML;
    }

    // Add interface as load balancer
    async addInterfaceAsLB(interfaceName, ip) {
        if (!ip || ip === 'No IP assigned') {
            this.showNotification('Interface has no IP address assigned', 'error');
            return;
        }

        const ratio = prompt('Enter contention ratio (1-100):', '1');
        if (!ratio || isNaN(ratio) || ratio < 1 || ratio > 100) {
            this.showNotification('Invalid contention ratio', 'error');
            return;
        }

        try {
            const response = await fetch('/api/lb/add', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    address: ip,
                    interface: interfaceName,
                    contention_ratio: parseInt(ratio),
                    tunnel_mode: false
                })
            });

            if (response.ok) {
                this.showNotification(`Added ${interfaceName} (${ip}) as load balancer`, 'success');
                this.loadLoadBalancers();
            } else {
                throw new Error('Failed to add load balancer');
            }
        } catch (error) {
            console.error('Failed to add load balancer:', error);
            this.showNotification('Failed to add load balancer', 'error');
        }
    }

    // Load active load balancers
    async loadLoadBalancers() {
        const container = document.getElementById('activeLoadBalancers');
        if (!container) return;
        
        container.innerHTML = '<div class="loading"><i class="fas fa-spinner fa-spin"></i>Loading load balancers...</div>';
        
        try {
            const response = await fetch('/api/config');
            if (response.ok) {
                const data = await response.json();
                this.activeLoadBalancers = data.load_balancers || [];
                this.renderLoadBalancers();
            } else {
                throw new Error('Failed to load load balancers');
            }
        } catch (error) {
            console.error('Failed to load load balancers:', error);
            container.innerHTML = '<div class="error-message"><i class="fas fa-exclamation-triangle"></i>Failed to load load balancers</div>';
        }
    }

    // Render active load balancers
    renderLoadBalancers() {
        const container = document.getElementById('activeLoadBalancers');
        if (!container) return;

        if (!this.activeLoadBalancers.length) {
            container.innerHTML = '<div class="error-message"><i class="fas fa-info-circle"></i>No load balancers configured</div>';
            return;
        }

        const lbHTML = this.activeLoadBalancers.map(lb => `
            <div class="load-balancer-item">
                <div class="load-balancer-info">
                    <div class="status-indicator ${lb.enabled ? '' : 'inactive'}"></div>
                    <div class="load-balancer-details">
                        <h4>LB${lb.id}: ${lb.address}</h4>
                        <p>Interface: ${lb.interface || 'N/A'} • Ratio: ${lb.contention_ratio} • Rules: ${Object.keys(lb.source_ip_rules || {}).length}</p>
                    </div>
                </div>
                <div class="load-balancer-actions">
                    <button class="btn btn-secondary btn-sm" onclick="settingsManager.editLoadBalancer('${lb.address}')">
                        <i class="fas fa-edit"></i>
                    </button>
                    <button class="btn btn-danger btn-sm" onclick="settingsManager.removeLoadBalancer('${lb.address}')">
                        <i class="fas fa-trash"></i>
                    </button>
                </div>
            </div>
        `).join('');

        container.innerHTML = lbHTML;
    }

    // Remove load balancer
    async removeLoadBalancer(address) {
        if (!confirm(`Are you sure you want to remove load balancer ${address}?`)) {
            return;
        }

        try {
            const response = await fetch('/api/lb/remove', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ address: address })
            });

            if (response.ok) {
                this.showNotification(`Removed load balancer ${address}`, 'success');
                this.loadLoadBalancers();
            } else {
                throw new Error('Failed to remove load balancer');
            }
        } catch (error) {
            console.error('Failed to remove load balancer:', error);
            this.showNotification('Failed to remove load balancer', 'error');
        }
    }

    // Load gateway configuration
    async loadGatewayConfig() {
        try {
            const response = await fetch('/api/gateway');
            if (response.ok) {
                const gatewayConfig = await response.json();
                this.populateGatewayForm(gatewayConfig);
            }
        } catch (error) {
            console.error('Failed to load gateway config:', error);
        }
    }

    // Populate gateway form
    populateGatewayForm(config) {
        const elements = {
            'gatewayEnabled': config.enabled || false,
            'gatewayIP': config.gateway_ip || '192.168.100.1',
            'subnetCIDR': config.subnet_cidr || '192.168.100.0/24',
            'natInterface': config.nat_interface || '',
            'transparentPort': config.transparent_port || 8888,
            'dnsPort': config.dns_port || 5353,
            'autoConfig': config.auto_configure || true,
            'dhcpStart': config.dhcp_range_start || '192.168.100.10',
            'dhcpEnd': config.dhcp_range_end || '192.168.100.100'
        };

        Object.entries(elements).forEach(([id, value]) => {
            const element = document.getElementById(id);
            if (element) {
                if (element.type === 'checkbox') {
                    element.checked = value;
                } else {
                    element.value = value;
                }
            }
        });
    }

    // Toggle gateway mode
    async toggleGatewayMode(enabled) {
        try {
            const response = await fetch('/api/gateway/toggle', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ enabled: enabled })
            });

            const result = await response.json();
            if (result.success) {
                this.showNotification(`Gateway mode ${enabled ? 'enabled' : 'disabled'}`, 'success');
            } else {
                this.showNotification(result.error || 'Failed to toggle gateway mode', 'error');
                // Revert checkbox
                document.getElementById('gatewayEnabled').checked = !enabled;
            }
        } catch (error) {
            console.error('Failed to toggle gateway mode:', error);
            this.showNotification('Failed to toggle gateway mode', 'error');
            document.getElementById('gatewayEnabled').checked = !enabled;
        }
    }

    // Save all settings
    async saveAllSettings() {
        const settings = this.collectAllSettings();
        
        try {
            console.log('Saving settings:', settings); // Debug log
            
            const response = await fetch('/api/settings', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(settings)
            });

            if (response.ok) {
                const result = await response.json();
                console.log('Save result:', result); // Debug log
                
                if (result.success) {
                    this.showNotification(result.message || 'Settings saved successfully', 'success');
                    
                    // Reload settings to reflect actual values
                    await this.loadSettings();
                    
                    // If there are restart-required settings, show restart option and button in settings-actions
                    if (result.requires_restart && result.requires_restart.length > 0) {
                        setTimeout(() => {
                            this.showNotificationWithAction(
                                `Settings saved! Restart required for some changes.`,
                                'warning',
                                'Restart',
                                () => this.restartService()
                            );
                            this.showRestartButton();
                        }, 1000);
                    } else {
                        this.hideRestartButton();
                    }
                } else {
                    this.showNotification(result.error || 'Failed to save settings', 'error');
                }
            } else {
                const errorText = await response.text();
                console.error('Server error:', errorText);
                throw new Error('Failed to save settings');
            }
        } catch (error) {
            console.error('Failed to save settings:', error);
            this.showNotification('Failed to save settings', 'error');
        }
    }

    // Collect all settings from form
    collectAllSettings() {
        return {
            // General settings
            listen_host: document.getElementById('lhost')?.value || '127.0.0.1',
            listen_port: parseInt(document.getElementById('lport')?.value) || 8080,
            web_port: parseInt(document.getElementById('webPort')?.value) || 0,
            tunnel_mode: document.getElementById('tunnel')?.checked || false,
            debug_mode: document.getElementById('debug')?.checked || false,
            
            // Gateway settings (flattened to match API expectations)
            gateway_mode: document.getElementById('gatewayEnabled')?.checked || false,
            gateway_ip: document.getElementById('gatewayIP')?.value || '192.168.100.1',
            subnet_cidr: document.getElementById('subnetCIDR')?.value || '192.168.100.0/24',
            nat_interface: document.getElementById('natInterface')?.value || '',
            transparent_port: parseInt(document.getElementById('transparentPort')?.value) || 8888,
            dns_port: parseInt(document.getElementById('dnsPort')?.value) || 5353,
            auto_configure: document.getElementById('autoConfig')?.checked || true,
            dhcp_start: document.getElementById('dhcpStart')?.value || '192.168.100.10',
            dhcp_end: document.getElementById('dhcpEnd')?.value || '192.168.100.100',
            
            // Performance settings
            max_connections: parseInt(document.getElementById('maxConnections')?.value) || 500,
            max_goroutines: parseInt(document.getElementById('maxGoroutines')?.value) || 1000
        };
    }

    // Show custom load balancer modal
    addCustomLoadBalancer() {
        this.showModal('addLBModal');
    }

    // Save custom load balancer
    async saveCustomLoadBalancer() {
        const address = document.getElementById('customLBAddress').value.trim();
        const ratio = parseInt(document.getElementById('customLBRatio').value) || 1;
        const tunnel = document.getElementById('customLBTunnel').checked;

        if (!address) {
            this.showNotification('Please enter an address', 'error');
            return;
        }

        try {
            const response = await fetch('/api/lb/add', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    address: address,
                    contention_ratio: ratio,
                    tunnel_mode: tunnel
                })
            });

            if (response.ok) {
                this.showNotification('Load balancer added successfully', 'success');
                this.closeModal('addLBModal');
                this.loadLoadBalancers();
                
                // Clear form
                document.getElementById('customLBAddress').value = '';
                document.getElementById('customLBRatio').value = '1';
                document.getElementById('customLBTunnel').checked = false;
            } else {
                throw new Error('Failed to add load balancer');
            }
        } catch (error) {
            console.error('Failed to add load balancer:', error);
            this.showNotification('Failed to add load balancer', 'error');
        }
    }



    // Reset to defaults
    async resetToDefaults() {
        if (!confirm('Are you sure you want to reset all settings to defaults? This cannot be undone.')) {
            return;
        }

        try {
            const response = await fetch('/api/reset', { method: 'POST' });
            if (response.ok) {
                this.showNotification('Settings reset to defaults', 'success');
                this.refreshSettings();
            } else {
                throw new Error('Failed to reset settings');
            }
        } catch (error) {
            console.error('Failed to reset settings:', error);
            this.showNotification('Failed to reset settings', 'error');
        }
    }

    // Restart service
    async restartService() {
        if (!confirm('Are you sure you want to restart the service? This will temporarily interrupt connections.')) {
            return;
        }

        try {
            const response = await fetch('/api/restart', { method: 'POST' });
            if (response.ok) {
                this.showNotification('Service restart initiated', 'success');
                // Wait a bit then reload page
                setTimeout(() => {
                    window.location.reload();
                }, 3000);
            } else {
                throw new Error('Failed to restart service');
            }
        } catch (error) {
            console.error('Failed to restart service:', error);
            this.showNotification('Failed to restart service', 'error');
        }
    }

    // Refresh all settings
    refreshSettings() {
        this.loadSettings();
        this.loadLoadBalancers();
        
        // Refresh current section
        const activeSection = document.querySelector('.settings-section.active');
        if (activeSection && activeSection.id === 'interfaces-settings') {
            this.scanInterfaces();
        } else if (activeSection && activeSection.id === 'gateway-settings') {
            this.loadGatewayConfig();
        }
    }

    // Modal management
    showModal(modalId) {
        const modal = document.getElementById(modalId);
        if (modal) {
            modal.style.display = 'block';
            document.body.style.overflow = 'hidden';
        }
    }

    closeModal(modalId) {
        const modal = document.getElementById(modalId);
        if (modal) {
            modal.style.display = 'none';
            document.body.style.overflow = 'auto';
        }
    }

    // Notification system
    showNotification(message, type = 'info') {
        // Remove existing notification
        const existing = document.querySelector('.notification');
        if (existing) {
            existing.remove();
        }

        const notification = document.createElement('div');
        notification.className = `notification notification-${type}`;
        notification.innerHTML = `
            <div class="notification-content">
                <i class="fas fa-${type === 'success' ? 'check-circle' : type === 'error' ? 'exclamation-circle' : 'info-circle'}"></i>
                <span>${message}</span>
            </div>
            <button class="notification-close" onclick="this.parentElement.remove()">
                <i class="fas fa-times"></i>
            </button>
        `;

        // Add notification styles
        Object.assign(notification.style, {
            position: 'fixed',
            top: '20px',
            right: '20px',
            zIndex: '10000',
            background: type === 'success' ? '#10b981' : type === 'error' ? '#ef4444' : type === 'warning' ? '#f59e0b' : '#3b82f6',
            color: 'white',
            padding: '16px 20px',
            borderRadius: '8px',
            boxShadow: '0 4px 12px rgba(0, 0, 0, 0.15)',
            display: 'flex',
            alignItems: 'center',
            gap: '12px',
            maxWidth: '400px',
            animation: 'slideInRight 0.3s ease-out'
        });

        document.body.appendChild(notification);

        // Auto-remove after 5 seconds
        setTimeout(() => {
            if (notification.parentElement) {
                notification.remove();
            }
        }, 5000);
    }

    // Notification with action button
    showNotificationWithAction(message, type = 'info', actionText = 'Action', actionCallback = null) {
        // Remove existing notification
        const existing = document.querySelector('.notification');
        if (existing) {
            existing.remove();
        }

        const notification = document.createElement('div');
        notification.className = `notification notification-${type}`;
        notification.innerHTML = `
            <div class="notification-content">
                <i class="fas fa-${type === 'success' ? 'check-circle' : type === 'error' ? 'exclamation-circle' : type === 'warning' ? 'exclamation-triangle' : 'info-circle'}"></i>
                <span>${message}</span>
            </div>
            <div class="notification-actions">
                <button class="notification-action-btn">${actionText}</button>
                <button class="notification-close" onclick="this.parentElement.parentElement.remove()">
                    <i class="fas fa-times"></i>
                </button>
            </div>
        `;

        // Add notification styles
        Object.assign(notification.style, {
            position: 'fixed',
            top: '20px',
            right: '20px',
            zIndex: '10000',
            background: type === 'success' ? '#10b981' : type === 'error' ? '#ef4444' : type === 'warning' ? '#f59e0b' : '#3b82f6',
            color: 'white',
            padding: '16px 20px',
            borderRadius: '8px',
            boxShadow: '0 4px 12px rgba(0, 0, 0, 0.15)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: '12px',
            maxWidth: '500px',
            animation: 'slideInRight 0.3s ease-out'
        });

        document.body.appendChild(notification);

        // Add action button event listener
        const actionBtn = notification.querySelector('.notification-action-btn');
        if (actionBtn && actionCallback) {
            actionBtn.addEventListener('click', () => {
                actionCallback();
                notification.remove();
            });
        }

        // Auto-remove after 10 seconds (longer for action notifications)
        setTimeout(() => {
            if (notification.parentElement) {
                notification.remove();
            }
        }, 10000);
    }

    // Show restart button in settings actions
    showRestartButton() {
        const actionsContainers = document.querySelectorAll('.settings-actions');
        actionsContainers.forEach(container => {
            container.style.display = 'flex';
            
            // Remove existing restart button if any
            const existingBtn = container.querySelector('.restart-service-btn');
            if (existingBtn) {
                existingBtn.remove();
            }
            
            // Add restart button
            const restartBtn = document.createElement('button');
            restartBtn.className = 'btn btn-warning restart-service-btn';
            restartBtn.innerHTML = '<i class="fas fa-power-off"></i> Restart Service';
            restartBtn.onclick = () => this.restartService();
            container.appendChild(restartBtn);
        });
    }

    // Hide restart button and settings actions if no changes
    hideRestartButton() {
        const actionsContainers = document.querySelectorAll('.settings-actions');
        actionsContainers.forEach(container => {
            const restartBtn = container.querySelector('.restart-service-btn');
            if (restartBtn) {
                restartBtn.remove();
            }
            
            // Hide container if no other buttons are needed
            if (container.children.length <= 2) { // Only save + refresh buttons
                container.style.display = 'none';
            }
        });
    }

    // Track if form has changes
    trackFormChanges() {
        const forms = document.querySelectorAll('#general-settings input, #gateway-settings input');
        forms.forEach(input => {
            input.addEventListener('change', () => {
                this.showSettingsActions();
            });
        });
    }

    // Show settings actions when changes are made
    showSettingsActions() {
        const actionsContainers = document.querySelectorAll('.settings-actions');
        actionsContainers.forEach(container => {
            container.style.display = 'flex';
        });
    }

    // Hide settings actions initially
    hideSettingsActions() {
        const actionsContainers = document.querySelectorAll('.settings-actions');
        actionsContainers.forEach(container => {
            container.style.display = 'none';
        });
    }
}

// Global functions for HTML onclick handlers
let settingsManager;

function showSettingsSection(sectionId, navElement) {
    settingsManager.showSettingsSection(sectionId, navElement);
}

function refreshSettings() {
    settingsManager.refreshSettings();
}

function saveAllSettings() {
    settingsManager.saveAllSettings();
}

function scanInterfaces() {
    settingsManager.scanInterfaces();
}

function addCustomLoadBalancer() {
    settingsManager.addCustomLoadBalancer();
}

function saveCustomLoadBalancer() {
    settingsManager.saveCustomLoadBalancer();
}

function toggleGatewayMode(enabled) {
    settingsManager.toggleGatewayMode(enabled);
}



function resetToDefaults() {
    settingsManager.resetToDefaults();
}

function restartService() {
    settingsManager.restartService();
}

function closeModal(modalId) {
    settingsManager.closeModal(modalId);
}

// Initialize when DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
    settingsManager = new SettingsManager();
    
    // Close modals when clicking outside
    window.addEventListener('click', (event) => {
        if (event.target.classList.contains('modal')) {
            event.target.style.display = 'none';
            document.body.style.overflow = 'auto';
        }
    });
    
    // Add CSS animation for notifications
    const style = document.createElement('style');
    style.textContent = `
        @keyframes slideInRight {
            from { transform: translateX(100%); opacity: 0; }
            to { transform: translateX(0); opacity: 1; }
        }
        .notification-content {
            display: flex;
            align-items: center;
            gap: 8px;
        }
        .notification-actions {
            display: flex;
            align-items: center;
            gap: 8px;
        }
        .notification-action-btn {
            background: rgba(255, 255, 255, 0.2);
            border: 1px solid rgba(255, 255, 255, 0.3);
            color: white;
            cursor: pointer;
            padding: 6px 12px;
            border-radius: 4px;
            font-size: 14px;
            font-weight: 500;
            transition: all 0.2s;
        }
        .notification-action-btn:hover {
            background: rgba(255, 255, 255, 0.3);
            border-color: rgba(255, 255, 255, 0.5);
        }
        .notification-close {
            background: none;
            border: none;
            color: inherit;
            cursor: pointer;
            padding: 4px;
            border-radius: 4px;
            transition: background 0.2s;
        }
        .notification-close:hover {
            background: rgba(0, 0, 0, 0.1);
        }
    `;
    document.head.appendChild(style);
}); 