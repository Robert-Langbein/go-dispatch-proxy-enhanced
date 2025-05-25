// Dashboard JavaScript - Real-time functionality

// Auto-refresh functionality
let autoRefreshInterval;
let trafficRefreshInterval;

function startAutoRefresh() {
    autoRefreshInterval = setInterval(() => {
        // Only refresh if user is not actively interacting
        if (document.visibilityState === 'visible' && !document.querySelector('.modal[style*="block"]')) {
            fetch('/api/stats')
                .then(response => response.json())
                .then(data => {
                    updateDashboard(data);
                })
                .catch(error => {
                    console.error('Auto-refresh failed:', error);
                });
        }
    }, 5000);
}

function stopAutoRefresh() {
    if (autoRefreshInterval) {
        clearInterval(autoRefreshInterval);
    }
}

// Update dashboard with new data
function updateDashboard(data) {
    // Update stats
    const statValues = document.querySelectorAll('.stat-value');
    if (statValues.length >= 4) {
        statValues[0].textContent = data.system_info.total_lbs;
        statValues[1].textContent = data.total_connections;
        statValues[2].textContent = data.overall_success_rate.toFixed(1) + '%';
        statValues[3].textContent = data.system_info.uptime;
    }
    
    // Update load balancer success rates
    data.load_balancers.forEach((lb, index) => {
        const lbCard = document.querySelectorAll('.lb-card')[index];
        if (lbCard) {
            const successRateElement = lbCard.querySelector('.lb-info p:nth-child(3)');
            if (successRateElement) {
                successRateElement.innerHTML = '<strong>Success Rate:</strong> ' + lb.success_rate.toFixed(1) + '%';
            }
            
            const statRow = lbCard.querySelector('.stat-row');
            if (statRow) {
                statRow.innerHTML = 
                    '<span>Total: ' + lb.total_connections + '</span>' +
                    '<span class="success">Success: ' + lb.success_count + '</span>' +
                    '<span class="error">Failures: ' + lb.failure_count + '</span>';
            }
        }
    });
}

// Connection filtering functionality
function filterConnections() {
    const sourceFilter = document.getElementById('sourceFilter').value.toLowerCase();
    const destFilter = document.getElementById('destFilter').value.toLowerCase();
    const rows = document.querySelectorAll('.connection-row');
    let visibleCount = 0;
    
    rows.forEach(row => {
        const sourceIP = row.getAttribute('data-source').toLowerCase();
        const destIP = row.getAttribute('data-dest').toLowerCase();
        
        const sourceMatch = sourceFilter === '' || sourceIP.includes(sourceFilter);
        const destMatch = destFilter === '' || destIP.includes(destFilter);
        
        if (sourceMatch && destMatch) {
            row.style.display = '';
            visibleCount++;
        } else {
            row.style.display = 'none';
        }
    });
    
    document.getElementById('connectionCount').textContent = visibleCount;
}

// Refresh connections manually
async function refreshConnections() {
    try {
        const response = await fetch('/api/connections');
        const data = await response.json();
        
        updateConnectionsTable(data.active_connections);
        document.getElementById('connectionCount').textContent = data.active_connections.length;
    } catch (error) {
        console.error('Error refreshing connections:', error);
    }
}

// Refresh entire dashboard
async function refreshDashboard() {
    try {
        const response = await fetch('/api/stats');
        const data = await response.json();
        updateDashboard(data);
        updateTrafficData();
        refreshConnections();
    } catch (error) {
        console.error('Error refreshing dashboard:', error);
    }
}

// Update connections table with new data
function updateConnectionsTable(connections) {
    const tbody = document.getElementById('connectionsBody');
    if (!tbody) return;
    
    tbody.innerHTML = '';
    
    connections.forEach(conn => {
        const row = document.createElement('tr');
        row.className = 'connection-row';
        row.setAttribute('data-source', conn.source_ip);
        row.setAttribute('data-dest', conn.destination_ip);
        
        const duration = formatDuration(new Date(conn.start_time));
        
        row.innerHTML = '' +
            '<td class="source-cell">' +
                '<span class="ip clickable-ip" onclick="showSourceIPManagement(\'' + conn.source_ip + '\')">' + conn.source_ip + '</span>' +
                '<span class="port">:' + conn.source_port + '</span>' +
            '</td>' +
            '<td class="dest-cell">' +
                '<span class="ip">' + conn.destination_ip + '</span>' +
                '<span class="port">:' + conn.destination_port + '</span>' +
            '</td>' +
            '<td class="lb-cell">LB' + (conn.lb_index + 1) + '</td>' +
            '<td class="duration-cell">' + duration + '</td>' +
            '<td class="traffic-cell">' +
                '<div class="traffic-info">' +
                    '<span class="bytes-in">↓' + formatBytes(conn.bytes_in) + '</span>' +
                    '<span class="bytes-out">↑' + formatBytes(conn.bytes_out) + '</span>' +
                '</div>' +
            '</td>' +
            '<td class="status-cell">' +
                '<span class="status ' + conn.status + '">' + conn.status + '</span>' +
            '</td>' +
            '<td class="actions-cell">' +
                '<button class="btn btn-small btn-primary" ' +
                        'onclick="showWeightModal(\'' + conn.source_ip + '\')">' +
                    'Set Weight' +
                '</button>' +
            '</td>';
        
        tbody.appendChild(row);
    });
}

// Enhanced auto-refresh with traffic data
function updateTrafficData() {
    fetch('/api/traffic')
        .then(response => response.json())
        .then(data => {
            // Update traffic statistics
            const bytesPerSecondEl = document.getElementById('bytesPerSecond');
            const totalDataTransferredEl = document.getElementById('totalDataTransferred');
            const activeConnectionsEl = document.getElementById('activeConnections');
            const connectionsPerMinuteEl = document.getElementById('connectionsPerMinute');
            
            if (bytesPerSecondEl) bytesPerSecondEl.textContent = formatBytes(data.bytes_per_second) + '/s';
            if (totalDataTransferredEl) totalDataTransferredEl.textContent = formatBytes(data.total_data_transferred);
            if (activeConnectionsEl) activeConnectionsEl.textContent = data.active_connections;
            if (connectionsPerMinuteEl) connectionsPerMinuteEl.textContent = data.connections_per_minute;
            
            // Update traffic bars
            updateTrafficBars();
        })
        .catch(error => {
            console.error('Error updating traffic data:', error);
        });
}

// Update traffic bars animation
function updateTrafficBars() {
    fetch('/api/stats')
        .then(response => response.json())
        .then(data => {
            const totalConnections = data.total_connections || 1;
            
            data.load_balancers.forEach((lb, index) => {
                const bar = document.querySelector('[data-lb="' + index + '"]');
                if (bar) {
                    const percentage = (lb.total_connections / totalConnections) * 100;
                    bar.style.width = percentage + '%';
                    const textEl = bar.querySelector('.traffic-bar-text');
                    if (textEl) {
                        textEl.textContent = lb.total_connections + ' connections (' + percentage.toFixed(1) + '%)';
                    }
                }
            });
        })
        .catch(error => {
            console.error('Error updating traffic bars:', error);
        });
}

// Enhanced auto-refresh functionality
function startTrafficRefresh() {
    trafficRefreshInterval = setInterval(() => {
        if (document.visibilityState === 'visible') {
            updateTrafficData();
            refreshConnections();
        }
    }, 2000); // Update every 2 seconds for real-time feel
}

function stopTrafficRefresh() {
    if (trafficRefreshInterval) {
        clearInterval(trafficRefreshInterval);
    }
}

// Modal Management Functions
function showModal(modalId) {
    const modal = document.getElementById(modalId);
    if (modal) {
        modal.style.display = 'block';
        stopAutoRefresh(); // Pause auto-refresh while modal is open
    }
}

function closeModal(modalId) {
    const modal = document.getElementById(modalId);
    if (modal) {
        modal.style.display = 'none';
        startAutoRefresh(); // Resume auto-refresh
    }
}

// Source IP Management
function showSourceIPManagement(sourceIP) {
    document.getElementById('sourceIPModalContent').innerHTML = 
        '<div class="loading">Loading rules for ' + sourceIP + '...</div>';
    
    showModal('sourceIPModal');
    
    // Load existing rules for this source IP
    fetch('/api/stats')
        .then(response => response.json())
        .then(data => {
            let content = '<div class="source-ip-management">';
            content += '<h4>Source IP: <code>' + sourceIP + '</code></h4>';
            content += '<p>Configure custom load balancing rules for this source IP.</p>';
            
            content += '<div class="lb-rules-grid">';
            data.load_balancers.forEach(lb => {
                const hasRule = lb.source_ip_rules && lb.source_ip_rules[sourceIP];
                const ratio = hasRule ? lb.source_ip_rules[sourceIP].contention_ratio : lb.default_ratio;
                
                content += '<div class="lb-rule-card">';
                content += '<h5>LB' + lb.id + ': ' + lb.address + '</h5>';
                content += '<p><strong>Interface:</strong> ' + lb.interface + '</p>';
                content += '<p><strong>Current Ratio:</strong> ' + ratio + '</p>';
                
                if (hasRule) {
                    content += '<p class="custom-rule">✓ Custom rule active</p>';
                    content += '<button class="btn btn-small btn-danger" onclick="removeSourceIPRule(\'' + 
                               lb.address + '\', \'' + sourceIP + '\')">Remove Rule</button>';
                } else {
                    content += '<p class="default-rule">Using default ratio</p>';
                }
                
                content += '<button class="btn btn-small btn-primary" onclick="showWeightModalForLB(\'' + 
                           sourceIP + '\', \'' + lb.address + '\')">Set Custom Ratio</button>';
                content += '</div>';
            });
            content += '</div>';
            
            content += '</div>';
            document.getElementById('sourceIPModalContent').innerHTML = content;
        })
        .catch(error => {
            document.getElementById('sourceIPModalContent').innerHTML = 
                '<div class="error">Error loading rules: ' + error.message + '</div>';
        });
}

// Add Rule Modal
function showAddRuleModal(lbAddress) {
    document.getElementById('modalLBAddress').value = lbAddress;
    document.getElementById('sourceIP').value = '';
    document.getElementById('contentionRatio').value = '1';
    document.getElementById('description').value = '';
    showModal('addRuleModal');
}

// Weight Modal
function showWeightModal(sourceIP) {
    document.getElementById('weightSourceIP').value = sourceIP;
    document.getElementById('weightSourceIPDisplay').textContent = sourceIP;
    document.getElementById('weightRatio').value = '1';
    document.getElementById('weightDescription').value = '';
    showModal('weightModal');
}

function showWeightModalForLB(sourceIP, lbAddress) {
    document.getElementById('weightSourceIP').value = sourceIP;
    document.getElementById('weightSourceIPDisplay').textContent = sourceIP;
    document.getElementById('weightLBSelect').value = lbAddress;
    document.getElementById('weightRatio').value = '1';
    document.getElementById('weightDescription').value = '';
    closeModal('sourceIPModal');
    showModal('weightModal');
}

// Load Balancer Toggle
async function toggleLoadBalancer(lbAddress, enabled) {
    try {
        const response = await fetch('/api/lb/toggle', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                lb_address: lbAddress,
                enabled: enabled
            })
        });
        
        const result = await response.json();
        if (result.success) {
            console.log('Load balancer toggled successfully');
            refreshDashboard();
        } else {
            console.error('Failed to toggle load balancer');
            // Revert checkbox state
            location.reload();
        }
    } catch (error) {
        console.error('Error toggling load balancer:', error);
        location.reload();
    }
}

// Remove Rule
async function removeRule(lbAddress, sourceIP) {
    if (!confirm('Remove rule for ' + sourceIP + ' on ' + lbAddress + '?')) {
        return;
    }
    
    try {
        const response = await fetch('/api/rules?lb_address=' + encodeURIComponent(lbAddress) + 
                                   '&source_ip=' + encodeURIComponent(sourceIP), {
            method: 'DELETE'
        });
        
        const result = await response.json();
        if (result.success) {
            console.log('Rule removed successfully');
            refreshDashboard();
        } else {
            alert('Failed to remove rule');
        }
    } catch (error) {
        console.error('Error removing rule:', error);
        alert('Error removing rule: ' + error.message);
    }
}

async function removeSourceIPRule(lbAddress, sourceIP) {
    await removeRule(lbAddress, sourceIP);
    closeModal('sourceIPModal');
    setTimeout(() => showSourceIPManagement(sourceIP), 500);
}

// Form Submissions
document.addEventListener('DOMContentLoaded', function() {
    // Add Rule Form
    const addRuleForm = document.getElementById('addRuleForm');
    if (addRuleForm) {
        addRuleForm.addEventListener('submit', async function(e) {
            e.preventDefault();
            
            const formData = new FormData(addRuleForm);
            const data = {
                lb_address: formData.get('lb_address'),
                source_ip: formData.get('source_ip'),
                contention_ratio: parseInt(formData.get('contention_ratio')),
                description: formData.get('description')
            };
            
            try {
                const response = await fetch('/api/rules', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify(data)
                });
                
                const result = await response.json();
                if (result.success) {
                    closeModal('addRuleModal');
                    refreshDashboard();
                } else {
                    alert('Failed to add rule');
                }
            } catch (error) {
                console.error('Error adding rule:', error);
                alert('Error adding rule: ' + error.message);
            }
        });
    }
    
    // Weight Form
    const weightForm = document.getElementById('weightForm');
    if (weightForm) {
        weightForm.addEventListener('submit', async function(e) {
            e.preventDefault();
            
            const formData = new FormData(weightForm);
            const data = {
                source_ip: formData.get('source_ip'),
                lb_address: formData.get('lb_address'),
                contention_ratio: parseInt(formData.get('contention_ratio')),
                description: formData.get('description')
            };
            
            try {
                const response = await fetch('/api/connection/weight', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify(data)
                });
                
                const result = await response.json();
                if (result.success) {
                    closeModal('weightModal');
                    refreshDashboard();
                } else {
                    alert('Failed to set weight');
                }
            } catch (error) {
                console.error('Error setting weight:', error);
                alert('Error setting weight: ' + error.message);
            }
        });
    }
    
    // Start auto-refresh
    startAutoRefresh();
    startTrafficRefresh();
});

// Close modals when clicking outside
window.addEventListener('click', function(event) {
    if (event.target.classList.contains('modal')) {
        event.target.style.display = 'none';
        startAutoRefresh();
    }
}); 