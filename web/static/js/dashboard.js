// Dashboard JavaScript - Real-time functionality

// Auto-refresh functionality
let autoRefreshInterval;
let trafficRefreshInterval;
let connectionsRefreshInterval;

// Table sorting state
let currentSortColumn = null;
let currentSortDirection = 'asc';

// Chart variables
let trafficChart;
let chartInitialized = false;
let chartData = {
    labels: [],
    datasets: [
        {
            label: 'Download Speed',
            data: [],
            borderColor: '#006fff',
            backgroundColor: 'rgba(0, 111, 255, 0.1)',
            tension: 0.4,
            fill: true,
            pointRadius: 0,
            pointHoverRadius: 4,
            borderWidth: 2
        },
        {
            label: 'Upload Speed',
            data: [],
            borderColor: '#00b8a9',
            backgroundColor: 'rgba(0, 184, 169, 0.1)',
            tension: 0.4,
            fill: true,
            pointRadius: 0,
            pointHoverRadius: 4,
            borderWidth: 2
        }
    ]
};

// Table sorting functionality
function initializeTableSorting() {
    // Add sorting to all tables with sortable headers
    const tables = document.querySelectorAll('.data-table');
    
    tables.forEach(table => {
        // Skip if already initialized
        if (table.hasAttribute('data-sorting-initialized')) {
            return;
        }
        
        const headers = table.querySelectorAll('th');
        headers.forEach((header, index) => {
            // Skip action columns (usually last column)
            if (!header.textContent.toLowerCase().includes('actions')) {
                header.classList.add('sortable');
                
                // Add sort indicator if not exists
                if (!header.querySelector('.sort-indicator')) {
                    const sortIndicator = document.createElement('span');
                    sortIndicator.className = 'sort-indicator';
                    sortIndicator.innerHTML = '<i class="fas fa-sort"></i>';
                    header.appendChild(sortIndicator);
                }
                
                // Add event listener
                header.addEventListener('click', () => {
                    sortTable(table, index, header);
                });
            }
        });
        
        // Mark table as initialized
        table.setAttribute('data-sorting-initialized', 'true');
    });
}

function sortTable(table, columnIndex, header) {
    // Determine sort direction
    let direction = 'asc';
    if (currentSortColumn === columnIndex && currentSortDirection === 'asc') {
        direction = 'desc';
    }
    
    // Update sort state
    currentSortColumn = columnIndex;
    currentSortDirection = direction;
    
    // Remove sorted class from all headers and reset indicators
    table.querySelectorAll('th').forEach(th => {
        th.classList.remove('sorted');
        const indicator = th.querySelector('.sort-indicator');
        if (indicator) {
            indicator.innerHTML = '<i class="fas fa-sort"></i>';
        }
    });
    
    // Add sorted class to current header and update indicator
    header.classList.add('sorted');
    const sortIndicator = header.querySelector('.sort-indicator');
    if (direction === 'asc') {
        sortIndicator.innerHTML = '<i class="fas fa-sort-up"></i>';
    } else {
        sortIndicator.innerHTML = '<i class="fas fa-sort-down"></i>';
    }
    
    // Apply the sort
    applySortToTable(table, columnIndex, header, direction);
}

function applySortToTable(table, columnIndex, header, direction) {
    const tbody = table.querySelector('tbody');
    const rows = Array.from(tbody.querySelectorAll('tr'));
    
    // Sort rows
    rows.sort((a, b) => {
        const aCell = a.cells[columnIndex];
        const bCell = b.cells[columnIndex];
        
        let aValue = getCellValue(aCell);
        let bValue = getCellValue(bCell);
        
        // Handle different data types
        if (isNumeric(aValue) && isNumeric(bValue)) {
            aValue = parseFloat(aValue);
            bValue = parseFloat(bValue);
        } else if (isDate(aValue) && isDate(bValue)) {
            aValue = new Date(aValue);
            bValue = new Date(bValue);
        } else {
            aValue = aValue.toLowerCase();
            bValue = bValue.toLowerCase();
        }
        
        if (direction === 'asc') {
            return aValue > bValue ? 1 : aValue < bValue ? -1 : 0;
        } else {
            return aValue < bValue ? 1 : aValue > bValue ? -1 : 0;
        }
    });
    
    // Re-append sorted rows
    rows.forEach(row => tbody.appendChild(row));
}

function getCellValue(cell) {
    // Extract text content, handling nested elements
    let value = cell.textContent || cell.innerText || '';
    
    // Clean up common patterns
    value = value.replace(/^\s+|\s+$/g, ''); // trim
    
    // Handle specific patterns
    if (value.includes('LB')) {
        // Extract LB number for load balancer sorting
        const match = value.match(/LB(\d+)/);
        return match ? parseInt(match[1]) : 0;
    }
    
    if (value.includes('%')) {
        // Extract percentage value
        return parseFloat(value.replace('%', ''));
    }
    
    if (value.match(/\d+(\.\d+)?\s*(B|KB|MB|GB|TB)/i)) {
        // Convert bytes to numeric value for sorting
        return convertBytesToNumber(value);
    }
    
    // Handle IP addresses for proper sorting
    if (value.match(/^\d+\.\d+\.\d+\.\d+/)) {
        const parts = value.split('.').map(part => parseInt(part.split(':')[0]));
        return parts[0] * 16777216 + parts[1] * 65536 + parts[2] * 256 + parts[3];
    }
    
    // Handle duration formats (e.g., "2m 30s", "1h 5m")
    if (value.match(/\d+[hms]/)) {
        let totalSeconds = 0;
        const hours = value.match(/(\d+)h/);
        const minutes = value.match(/(\d+)m/);
        const seconds = value.match(/(\d+)s/);
        
        if (hours) totalSeconds += parseInt(hours[1]) * 3600;
        if (minutes) totalSeconds += parseInt(minutes[1]) * 60;
        if (seconds) totalSeconds += parseInt(seconds[1]);
        
        return totalSeconds;
    }
    
    // Handle numeric values
    const numericMatch = value.match(/^[\d,]+(\.\d+)?/);
    if (numericMatch) {
        return parseFloat(numericMatch[0].replace(/,/g, ''));
    }
    
    return value.toLowerCase();
}

function isNumeric(value) {
    return !isNaN(parseFloat(value)) && isFinite(value);
}

function isDate(value) {
    return !isNaN(Date.parse(value));
}

function convertBytesToNumber(bytesString) {
    const units = {
        'B': 1,
        'KB': 1024,
        'MB': 1024 * 1024,
        'GB': 1024 * 1024 * 1024,
        'TB': 1024 * 1024 * 1024 * 1024
    };
    
    // Handle various formats like "1.5 MB", "1024B", "2.3 GB"
    const match = bytesString.match(/([\d.,]+)\s*([KMGT]?B)/i);
    if (match) {
        const value = parseFloat(match[1].replace(/,/g, ''));
        const unit = match[2].toUpperCase();
        return value * (units[unit] || 1);
    }
    
    return 0;
}

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
    
    // Store current sort state before updating
    const table = tbody.closest('table');
    const currentSortedHeader = table ? table.querySelector('th.sorted') : null;
    let sortColumnIndex = -1;
    let sortDirection = 'asc';
    
    if (currentSortedHeader) {
        const headers = Array.from(table.querySelectorAll('th'));
        sortColumnIndex = headers.indexOf(currentSortedHeader);
        const sortIndicator = currentSortedHeader.querySelector('.sort-indicator i');
        if (sortIndicator && sortIndicator.classList.contains('fa-sort-down')) {
            sortDirection = 'desc';
        }
    }
    
    tbody.innerHTML = '';
    
    connections.forEach(conn => {
        const row = document.createElement('tr');
        row.className = 'connection-row';
        row.setAttribute('data-source', conn.source_ip);
        row.setAttribute('data-dest', conn.destination_ip);
        
        const duration = formatDuration(new Date(conn.start_time));
        
        row.innerHTML = '' +
            '<td>' +
                '<div class="d-flex align-items-center">' +
                    '<span class="font-weight-bold text-primary" onclick="showSourceIPManagement(\'' + conn.source_ip + '\')" style="cursor: pointer;">' + conn.source_ip + '</span>' +
                    '<span class="text-tertiary">:' + conn.source_port + '</span>' +
                '</div>' +
            '</td>' +
            '<td>' +
                '<div class="d-flex align-items-center">' +
                    '<span class="font-weight-bold">' + conn.destination_ip + '</span>' +
                    '<span class="text-tertiary">:' + conn.destination_port + '</span>' +
                '</div>' +
            '</td>' +
            '<td>' +
                '<div class="d-flex align-items-center">' +
                    '<i class="fas fa-server text-tertiary"></i>' +
                    '<span class="ml-2">LB' + (conn.lb_index + 1) + '</span>' +
                '</div>' +
            '</td>' +
            '<td>' +
                '<div class="d-flex align-items-center">' +
                    '<i class="fas fa-stopwatch text-tertiary"></i>' +
                    '<span class="ml-2">' + duration + '</span>' +
                '</div>' +
            '</td>' +
            '<td>' +
                '<div class="d-flex flex-column">' +
                    '<span class="text-success">' +
                        '<i class="fas fa-arrow-down"></i>' +
                        formatBytes(conn.bytes_in) +
                    '</span>' +
                    '<span class="text-info">' +
                        '<i class="fas fa-arrow-up"></i>' +
                        formatBytes(conn.bytes_out) +
                    '</span>' +
                '</div>' +
            '</td>' +
            '<td>' +
                '<div class="d-flex align-items-center">' +
                    '<div class="status-ball ' + conn.status + '"></div>' +
                    '<span class="text-' + conn.status + '">' + conn.status + '</span>' +
                '</div>' +
            '</td>' +
            '<td>' +
                '<button class="btn btn-sm btn-primary" onclick="showWeightModal(\'' + conn.source_ip + '\')">' +
                    '<i class="fas fa-weight-hanging"></i>' +
                    'Set Weight' +
                '</button>' +
            '</td>';
        
        tbody.appendChild(row);
    });
    
    // Initialize table sorting if not already done
    if (table && !table.hasAttribute('data-sorting-initialized')) {
        initializeTableSorting();
    }
    
    // Restore sort state if it existed
    if (sortColumnIndex >= 0 && currentSortedHeader) {
        currentSortColumn = sortColumnIndex;
        currentSortDirection = sortDirection;
        applySortToTable(table, sortColumnIndex, currentSortedHeader, sortDirection);
    }
}

// Enhanced auto-refresh with traffic data
function updateTrafficData() {
    fetch('/api/traffic')
        .then(response => response.json())
        .then(data => {
            // Update traffic statistics (but NOT the separate speed displays)
            const totalDataTransferredEl = document.getElementById('totalDataTransferred');
            const activeConnectionsEl = document.getElementById('activeConnections');
            const connectionsPerSecondEl = document.getElementById('connectionsPerSecond');
            
            // Note: Combined, upload, and download speeds are updated in updateSeparateSpeedDisplays()
            if (totalDataTransferredEl) totalDataTransferredEl.textContent = formatBytes(data.total_data_transferred);
            if (activeConnectionsEl) activeConnectionsEl.textContent = data.active_connections;
            if (connectionsPerSecondEl) connectionsPerSecondEl.textContent = data.connections_per_second;
            
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
                    
                    // Update the traffic stats display
                    const trafficBar = bar.closest('.lb-traffic-bar');
                    if (trafficBar) {
                        const statsEl = trafficBar.querySelector('.lb-traffic-stats');
                        if (statsEl) {
                            statsEl.textContent = lb.total_connections + ' connections (' + percentage.toFixed(1) + '%)';
                        }
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
            updateTrafficChart();
        }
    }, 500); // Update every 0.5 seconds for real-time feel
    
    // Separate interval for connections to avoid disrupting sorting
    connectionsRefreshInterval = setInterval(() => {
        if (document.visibilityState === 'visible') {
            refreshConnections();
        }
    }, 3000); // Update connections every 3 seconds
}

function stopTrafficRefresh() {
    if (trafficRefreshInterval) {
        clearInterval(trafficRefreshInterval);
    }
    if (connectionsRefreshInterval) {
        clearInterval(connectionsRefreshInterval);
    }
}

// Theme Toggle Functionality
function toggleTheme() {
    const currentTheme = document.documentElement.getAttribute('data-theme');
    const newTheme = currentTheme === 'dark' ? 'light' : 'dark';
    
    document.documentElement.setAttribute('data-theme', newTheme);
    localStorage.setItem('theme', newTheme);
    
    // Update toggle button
    const themeIcon = document.querySelector('.theme-icon');
    const themeText = document.querySelector('.theme-text');
    
    if (newTheme === 'dark') {
        themeIcon.className = 'fas fa-sun theme-icon';
        themeText.textContent = 'Light';
    } else {
        themeIcon.className = 'fas fa-moon theme-icon';
        themeText.textContent = 'Dark';
    }
}

// Initialize theme on page load
function initializeTheme() {
    const savedTheme = localStorage.getItem('theme') || 'light';
    document.documentElement.setAttribute('data-theme', savedTheme);
    
    // Update toggle button if it exists
    const themeIcon = document.querySelector('.theme-icon');
    const themeText = document.querySelector('.theme-text');
    
    if (themeIcon && themeText) {
        if (savedTheme === 'dark') {
            themeIcon.className = 'fas fa-sun theme-icon';
            themeText.textContent = 'Light';
        } else {
            themeIcon.className = 'fas fa-moon theme-icon';
            themeText.textContent = 'Dark';
        }
    }
}

// Modal Management Functions
function showModal(modalId) {
    const modal = document.getElementById(modalId);
    if (modal) {
        modal.style.display = 'block';
        stopAutoRefresh(); // Pause auto-refresh while modal is open
        stopTrafficRefresh(); // Also pause traffic refresh
    }
}

function closeModal(modalId) {
    const modal = document.getElementById(modalId);
    if (modal) {
        modal.style.display = 'none';
        startAutoRefresh(); // Resume auto-refresh
        startTrafficRefresh(); // Resume traffic refresh
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

// Add missing functions for template compatibility
async function addSourceIPRule() {
    const form = document.getElementById('addRuleForm');
    const formData = new FormData(form);
    
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
}

async function setWeight() {
    const form = document.getElementById('weightForm');
    const formData = new FormData(form);
    
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
}

function showRulesModal(lbAddress) {
    // Find the load balancer data
    fetch('/api/stats')
        .then(response => response.json())
        .then(data => {
            const lb = data.load_balancers.find(l => l.address === lbAddress);
            if (!lb) {
                alert('Load balancer not found');
                return;
            }
            
            let content = '<h4>Source IP Rules for LB: ' + lbAddress + '</h4>';
            
            if (lb.source_ip_rules && Object.keys(lb.source_ip_rules).length > 0) {
                content += '<table class="data-table">';
                content += '<thead><tr><th>Source IP</th><th>Ratio</th><th>Description</th><th>Actions</th></tr></thead>';
                content += '<tbody>';
                
                Object.entries(lb.source_ip_rules).forEach(([sourceIP, rule]) => {
                    content += '<tr>';
                    content += '<td>' + sourceIP + '</td>';
                    content += '<td>' + rule.contention_ratio + '</td>';
                    content += '<td>' + (rule.description || 'No description') + '</td>';
                    content += '<td>';
                    content += '<button class="btn btn-sm btn-danger" onclick="removeRule(\'' + lbAddress + '\', \'' + sourceIP + '\')">Remove</button>';
                    content += '</td>';
                    content += '</tr>';
                });
                
                content += '</tbody></table>';
            } else {
                content += '<p>No custom rules defined for this load balancer.</p>';
            }
            
            document.getElementById('rulesContent').innerHTML = content;
            showModal('viewRulesModal');
        })
        .catch(error => {
            console.error('Error loading rules:', error);
            alert('Error loading rules: ' + error.message);
        });
}

// Form Submissions
// Initialize Traffic Chart
function initializeTrafficChart() {
    const ctx = document.getElementById('trafficChart');
    if (!ctx || chartInitialized) {
        console.log('Traffic chart canvas not found or already initialized');
        return;
    }
    
    try {
        // Destroy existing chart if any
        if (trafficChart) {
            trafficChart.destroy();
        }
        
        trafficChart = new Chart(ctx, {
            type: 'line',
            data: chartData,
            options: {
                responsive: true,
                maintainAspectRatio: false,
                resizeDelay: 0,
                animation: {
                    duration: 0 // Disable animations for real-time updates
                },
                plugins: {
                    title: {
                        display: true,
                        text: 'Real-time Upload/Download Speeds',
                        color: '#212327',
                        font: {
                            size: 16
                        }
                    },
                    legend: {
                        position: 'top',
                        labels: {
                            color: '#212327',
                            usePointStyle: true,
                            padding: 20
                        }
                    }
                },
                scales: {
                    x: {
                        type: 'linear',
                        display: true,
                        title: {
                            display: true,
                            text: 'Time (seconds ago)',
                            color: '#50565e'
                        },
                        grid: {
                            color: 'rgba(80, 86, 94, 0.2)'
                        },
                        ticks: {
                            color: '#50565e',
                            stepSize: 10,
                            callback: function(value) {
                                const seconds = Math.floor((60 - value) / 2); // Convert index to seconds ago
                                return seconds + 's';
                            }
                        }
                    },
                    y: {
                        beginAtZero: true,
                        title: {
                            display: true,
                            text: 'Speed (MB/s)',
                            color: '#50565e'
                        },
                        grid: {
                            color: 'rgba(80, 86, 94, 0.2)'
                        },
                        ticks: {
                            color: '#50565e',
                            callback: function(value) {
                                if (value === 0) return '0';
                                if (value < 1) return (value * 1000).toFixed(0) + ' KB/s';
                                return value.toFixed(1) + ' MB/s';
                            }
                        }
                    }
                },
                interaction: {
                    intersect: false,
                    mode: 'index'
                },
                elements: {
                    point: {
                        radius: 0,
                        hoverRadius: 4
                    }
                }
            }
        });
        
        chartInitialized = true;
        console.log('Traffic chart initialized successfully');
    } catch (error) {
        console.error('Error initializing traffic chart:', error);
        chartInitialized = false;
    }
}

// Update Traffic Chart and separate speeds
function updateTrafficChart() {
    fetch('/api/traffic')
        .then(response => response.json())
        .then(data => {
            // Update separate upload/download speeds in real-time
            updateSeparateSpeedDisplays(data);
            
            // Update chart if available
            if (trafficChart) {
                updateChartData(data);
            }
        })
        .catch(error => {
            console.error('Error updating traffic data:', error);
        });
}

// Update separate speed displays with stacked format
function updateSeparateSpeedDisplays(data) {
    // Update upload speed with stacked display
    const uploadSpeedEl = document.getElementById('uploadSpeed');
    const uploadSpeedBitsEl = document.getElementById('uploadSpeedBits');
    
    if (uploadSpeedEl && data.bytes_out_per_second !== undefined) {
        const bytesOutRate = formatBytes(data.bytes_out_per_second) + '/s';
        uploadSpeedEl.textContent = bytesOutRate;
    }
    
    if (uploadSpeedBitsEl && data.bytes_out_per_second !== undefined) {
        const bitsOutRate = formatBits(data.bytes_out_per_second * 8) + 'bit/s';
        uploadSpeedBitsEl.textContent = bitsOutRate;
    }
    
    // Update download speed with stacked display
    const downloadSpeedEl = document.getElementById('downloadSpeed');
    const downloadSpeedBitsEl = document.getElementById('downloadSpeedBits');
    
    if (downloadSpeedEl && data.bytes_in_per_second !== undefined) {
        const bytesInRate = formatBytes(data.bytes_in_per_second) + '/s';
        downloadSpeedEl.textContent = bytesInRate;
    }
    
    if (downloadSpeedBitsEl && data.bytes_in_per_second !== undefined) {
        const bitsInRate = formatBits(data.bytes_in_per_second * 8) + 'bit/s';
        downloadSpeedBitsEl.textContent = bitsInRate;
    }
    
    // Update combined speed with stacked display
    const bytesPerSecondEl = document.getElementById('bytesPerSecond');
    const combinedSpeedBitsEl = document.querySelector('#bytesPerSecond').parentElement.querySelector('.speed-bits');
    
    if (bytesPerSecondEl && data.bytes_per_second !== undefined) {
        const combinedRate = formatBytes(data.bytes_per_second) + '/s';
        bytesPerSecondEl.textContent = combinedRate;
    }
    
    if (combinedSpeedBitsEl && data.bytes_per_second !== undefined) {
        const combinedBitsRate = formatBits(data.bytes_per_second * 8) + 'bit/s';
        combinedSpeedBitsEl.textContent = combinedBitsRate;
    }
}

// Update chart data
function updateChartData(data) {
    if (!trafficChart || !chartInitialized) {
        console.log('Chart not initialized, skipping update');
        return;
    }
    
    // Convert bytes to MB for chart display
    const downloadSpeedMB = (data.bytes_in_per_second || 0) / (1024 * 1024);
    const uploadSpeedMB = (data.bytes_out_per_second || 0) / (1024 * 1024);
    
    // Add new data point
    const dataIndex = chartData.labels.length;
    chartData.labels.push(dataIndex);
    chartData.datasets[0].data.push(downloadSpeedMB);
    chartData.datasets[1].data.push(uploadSpeedMB);
    
    // Keep only last 60 data points (30 seconds at 0.5s intervals)
    const maxPoints = 60;
    if (chartData.labels.length > maxPoints) {
        chartData.labels.shift();
        chartData.datasets[0].data.shift();
        chartData.datasets[1].data.shift();
        
        // Reindex labels for proper display
        for (let i = 0; i < chartData.labels.length; i++) {
            chartData.labels[i] = i;
        }
    }
    
    // Update chart without animation for real-time feel
    try {
        trafficChart.update('none');
    } catch (error) {
        console.error('Error updating chart:', error);
        // Try to reinitialize on error
        chartInitialized = false;
        setTimeout(() => {
            initializeTrafficChart();
        }, 1000);
    }
}

document.addEventListener('DOMContentLoaded', function() {
    console.log('Dashboard DOM loaded, initializing...');
    
    // Initialize theme first
    initializeTheme();
    
    // Initialize table sorting
    initializeTableSorting();
    
    // Wait for Chart.js to be fully loaded
    if (typeof Chart !== 'undefined') {
        console.log('Chart.js is available, initializing chart...');
        initializeTrafficChart();
    } else {
        console.log('Chart.js not yet available, waiting...');
        setTimeout(() => {
            if (typeof Chart !== 'undefined') {
                initializeTrafficChart();
            } else {
                console.error('Chart.js failed to load');
            }
        }, 1000);
    }
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
        startTrafficRefresh();
    }
}); 