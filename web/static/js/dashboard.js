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
                '<span class="ip">' + conn.source_ip + '</span>' +
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
                        'onclick="showWeightModal(\'' + conn.source_ip + '\', \'' + conn.load_balancer + '\')">' +
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
            
            if (bytesPerSecondEl) bytesPerSecondEl.textContent = formatBytes(data.bytes_per_second);
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
                        textEl.textContent = lb.total_connections + ' connections';
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

// Initialize auto-refresh when page loads
document.addEventListener('DOMContentLoaded', function() {
    startAutoRefresh();
    startTrafficRefresh();
});

// Stop auto-refresh when page is hidden
document.addEventListener('visibilitychange', function() {
    if (document.visibilityState === 'hidden') {
        stopAutoRefresh();
        stopTrafficRefresh();
    } else {
        startAutoRefresh();
        startTrafficRefresh();
    }
}); 