// web_templates.go
package main

/*
Get login HTML template
*/
func getLoginHTML(errorMsg string) string {
	errorDiv := ""
	if errorMsg != "" {
		errorDiv = `<div class="error">` + errorMsg + `</div>`
	}
	
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Go Dispatch Proxy Enhanced - Login</title>
    <link rel="stylesheet" href="/static/style.css">
</head>
<body class="login-page">
    <div class="login-container">
        <div class="login-box">
            <h1>🚀 Go Dispatch Proxy Enhanced</h1>
            <p>Enhanced Load Balancing Web Interface</p>
            ` + errorDiv + `
            <form method="POST" action="/login">
                <div class="form-group">
                    <label for="username">Username:</label>
                    <input type="text" id="username" name="username" required autofocus>
                </div>
                <div class="form-group">
                    <label for="password">Password:</label>
                    <input type="password" id="password" name="password" required>
                </div>
                <button type="submit" class="btn btn-primary">Login</button>
            </form>
            <div class="info">
                <small>Default credentials: admin/admin<br>
                Set WEB_USERNAME and WEB_PASSWORD environment variables to customize</small>
            </div>
        </div>
    </div>
</body>
</html>`
}

/*
Get dashboard HTML template
*/
func getDashboardHTML() string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Go Dispatch Proxy Enhanced - Dashboard</title>
    <link rel="stylesheet" href="/static/style.css">
    <script src="/static/script.js"></script>
</head>
<body>
    <nav class="navbar">
        <div class="nav-brand">
            <h1>🚀 Go Dispatch Proxy Enhanced</h1>
        </div>
        <div class="nav-controls">
            <span class="status {{if gt .SystemInfo.TotalLBs 0}}online{{else}}offline{{end}}">
                {{if gt .SystemInfo.TotalLBs 0}}● Online{{else}}● Offline{{end}}
            </span>
            <a href="/logout" class="btn btn-secondary">Logout</a>
        </div>
    </nav>

    <div class="container">
        <!-- System Overview -->
        <div class="section">
            <h2>📊 System Overview</h2>
            <div class="stats-grid">
                <div class="stat-card">
                    <div class="stat-value">{{.SystemInfo.TotalLBs}}</div>
                    <div class="stat-label">Load Balancers</div>
                </div>
                <div class="stat-card">
                    <div class="stat-value">{{.TotalConnections}}</div>
                    <div class="stat-label">Total Connections</div>
                </div>
                <div class="stat-card">
                    <div class="stat-value">{{printf "%.1f%%" .OverallSuccessRate}}</div>
                    <div class="stat-label">Success Rate</div>
                </div>
                <div class="stat-card">
                    <div class="stat-value">{{.SystemInfo.Uptime}}</div>
                    <div class="stat-label">Uptime</div>
                </div>
            </div>
        </div>

        <!-- Load Balancers -->
        <div class="section">
            <h2>⚖️ Load Balancers</h2>
            <div class="lb-grid">
                {{range .LoadBalancers}}
                <div class="lb-card {{if .Enabled}}enabled{{else}}disabled{{end}}">
                    <div class="lb-header">
                        <h3>LB {{.ID}}: {{.Address}}</h3>
                        <label class="toggle">
                            <input type="checkbox" {{if .Enabled}}checked{{end}} 
                                   onchange="toggleLoadBalancer('{{.Address}}', this.checked)">
                            <span class="slider"></span>
                        </label>
                    </div>
                    <div class="lb-info">
                        <p><strong>Interface:</strong> {{.Interface}}</p>
                        <p><strong>Default Ratio:</strong> {{.DefaultRatio}}</p>
                        <p><strong>Success Rate:</strong> {{printf "%.1f%%" .SuccessRate}}</p>
                    </div>
                    <div class="lb-stats">
                        <div class="stat-row">
                            <span>Total: {{.TotalConnections}}</span>
                            <span class="success">Success: {{.SuccessCount}}</span>
                            <span class="error">Failures: {{.FailureCount}}</span>
                        </div>
                    </div>
                    {{if .SourceIPRules}}
                    <div class="source-rules">
                        <h4>Source IP Rules ({{len .SourceIPRules}})</h4>
                        {{range $ip, $rule := .SourceIPRules}}
                        <div class="rule-item">
                            <span class="source-ip">{{$ip}}</span>
                            <span class="ratio">Ratio: {{$rule.ContentionRatio}}</span>
                            <button class="btn btn-small btn-danger" 
                                    onclick="removeRule('{{$.Address}}', '{{$ip}}')">Remove</button>
                        </div>
                        {{end}}
                    </div>
                    {{end}}
                    <div class="add-rule">
                        <button class="btn btn-small btn-primary" 
                                onclick="showAddRuleModal('{{.Address}}')">Add Source IP Rule</button>
                    </div>
                </div>
                {{end}}
            </div>
        </div>

        <!-- Active Source IPs -->
        {{if .ActiveSources}}
        <div class="section">
            <h2>🌐 Active Source IPs</h2>
            <div class="sources-table">
                <table>
                    <thead>
                        <tr>
                            <th>Source IP</th>
                            <th>Active Connections</th>
                            <th>Assigned LB</th>
                            <th>Effective Ratio</th>
                        </tr>
                    </thead>
                    <tbody>
                        {{range .ActiveSources}}
                        <tr>
                            <td>{{.SourceIP}}</td>
                            <td>{{.ActiveConnections}}</td>
                            <td>{{.AssignedLB}}</td>
                            <td>{{.EffectiveRatio}}</td>
                        </tr>
                        {{end}}
                    </tbody>
                </table>
            </div>
        </div>
        {{end}}

        <!-- Configuration -->
        <div class="section">
            <h2>⚙️ Configuration</h2>
            <div class="config-info">
                <p><strong>Version:</strong> {{.SystemInfo.Version}}</p>
                <p><strong>Config File:</strong> {{.SystemInfo.ConfigFile}}</p>
                <p><strong>Listen Address:</strong> {{.SystemInfo.ListenAddress}}</p>
                <p><strong>Started:</strong> {{.SystemInfo.StartTime.Format "2006-01-02 15:04:05"}}</p>
            </div>
        </div>
    </div>

    <!-- Add Rule Modal -->
    <div id="addRuleModal" class="modal">
        <div class="modal-content">
            <span class="close" onclick="closeModal()">&times;</span>
            <h3>Add Source IP Rule</h3>
            <form id="addRuleForm">
                <input type="hidden" id="modalLBAddress" name="lb_address">
                <div class="form-group">
                    <label for="sourceIP">Source IP/CIDR:</label>
                    <input type="text" id="sourceIP" name="source_ip" required 
                           placeholder="192.168.1.100 or 10.0.0.0/24">
                </div>
                <div class="form-group">
                    <label for="contentionRatio">Contention Ratio:</label>
                    <input type="number" id="contentionRatio" name="contention_ratio" 
                           min="1" max="100" value="1" required>
                </div>
                <div class="form-group">
                    <label for="description">Description:</label>
                    <input type="text" id="description" name="description" 
                           placeholder="e.g., High priority client">
                </div>
                <div class="form-actions">
                    <button type="button" class="btn btn-secondary" onclick="closeModal()">Cancel</button>
                    <button type="submit" class="btn btn-primary">Add Rule</button>
                </div>
            </form>
        </div>
    </div>

    <script>
        // Auto-refresh every 5 seconds
        setInterval(function() {
            location.reload();
        }, 5000);
    </script>
</body>
</html>`
}

/*
Get CSS styles
*/
func getCSS() string {
	return `
/* Reset and Base Styles */
* {
    margin: 0;
    padding: 0;
    box-sizing: border-box;
}

body {
    font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
    background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
    color: #333;
    line-height: 1.6;
    min-height: 100vh;
}

/* Login Page Styles */
.login-page {
    display: flex;
    align-items: center;
    justify-content: center;
    min-height: 100vh;
}

.login-container {
    width: 100%;
    max-width: 400px;
    padding: 20px;
}

.login-box {
    background: white;
    padding: 40px;
    border-radius: 10px;
    box-shadow: 0 10px 30px rgba(0, 0, 0, 0.3);
    text-align: center;
}

.login-box h1 {
    color: #667eea;
    margin-bottom: 10px;
    font-size: 24px;
}

.login-box p {
    color: #666;
    margin-bottom: 30px;
}

.error {
    background: #fee;
    color: #c33;
    padding: 10px;
    border-radius: 5px;
    margin-bottom: 20px;
    border: 1px solid #fcc;
}

/* Form Styles */
.form-group {
    margin-bottom: 20px;
    text-align: left;
}

.form-group label {
    display: block;
    margin-bottom: 5px;
    color: #555;
    font-weight: 500;
}

.form-group input {
    width: 100%;
    padding: 12px;
    border: 2px solid #ddd;
    border-radius: 5px;
    font-size: 16px;
    transition: border-color 0.3s;
}

.form-group input:focus {
    outline: none;
    border-color: #667eea;
}

/* Button Styles */
.btn {
    padding: 12px 24px;
    border: none;
    border-radius: 5px;
    cursor: pointer;
    font-size: 14px;
    font-weight: 500;
    text-decoration: none;
    display: inline-block;
    transition: background-color 0.3s;
}

.btn-primary {
    background: #667eea;
    color: white;
}

.btn-primary:hover {
    background: #5a6fd8;
}

.btn-secondary {
    background: #6c757d;
    color: white;
}

.btn-secondary:hover {
    background: #5a6268;
}

.btn-danger {
    background: #dc3545;
    color: white;
}

.btn-danger:hover {
    background: #c82333;
}

.btn-small {
    padding: 6px 12px;
    font-size: 12px;
}

/* Dashboard Styles */
.navbar {
    background: white;
    padding: 15px 30px;
    box-shadow: 0 2px 10px rgba(0, 0, 0, 0.1);
    display: flex;
    justify-content: space-between;
    align-items: center;
}

.nav-brand h1 {
    color: #667eea;
    font-size: 20px;
}

.nav-controls {
    display: flex;
    align-items: center;
    gap: 20px;
}

.status {
    font-weight: 500;
    padding: 6px 12px;
    border-radius: 20px;
    font-size: 14px;
}

.status.online {
    background: #d4edda;
    color: #155724;
}

.status.offline {
    background: #f8d7da;
    color: #721c24;
}

.container {
    max-width: 1200px;
    margin: 0 auto;
    padding: 30px 20px;
}

.section {
    background: white;
    margin-bottom: 30px;
    padding: 30px;
    border-radius: 10px;
    box-shadow: 0 5px 15px rgba(0, 0, 0, 0.1);
}

.section h2 {
    color: #333;
    margin-bottom: 20px;
    font-size: 20px;
}

/* Stats Grid */
.stats-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
    gap: 20px;
}

.stat-card {
    background: linear-gradient(135deg, #667eea, #764ba2);
    color: white;
    padding: 25px;
    border-radius: 10px;
    text-align: center;
}

.stat-value {
    font-size: 32px;
    font-weight: bold;
    margin-bottom: 5px;
}

.stat-label {
    font-size: 14px;
    opacity: 0.9;
}

/* Load Balancer Grid */
.lb-grid {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(350px, 1fr));
    gap: 20px;
}

.lb-card {
    border: 2px solid #ddd;
    border-radius: 10px;
    padding: 20px;
    transition: border-color 0.3s;
}

.lb-card.enabled {
    border-color: #28a745;
    background: #f8fff9;
}

.lb-card.disabled {
    border-color: #dc3545;
    background: #fff8f8;
    opacity: 0.7;
}

.lb-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 15px;
}

.lb-header h3 {
    font-size: 16px;
    color: #333;
}

/* Toggle Switch */
.toggle {
    position: relative;
    display: inline-block;
    width: 50px;
    height: 25px;
}

.toggle input {
    opacity: 0;
    width: 0;
    height: 0;
}

.slider {
    position: absolute;
    cursor: pointer;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background-color: #ccc;
    transition: .4s;
    border-radius: 25px;
}

.slider:before {
    position: absolute;
    content: "";
    height: 19px;
    width: 19px;
    left: 3px;
    bottom: 3px;
    background-color: white;
    transition: .4s;
    border-radius: 50%;
}

input:checked + .slider {
    background-color: #28a745;
}

input:checked + .slider:before {
    transform: translateX(25px);
}

/* Load Balancer Info */
.lb-info p {
    margin-bottom: 8px;
    font-size: 14px;
}

.lb-stats {
    margin: 15px 0;
}

.stat-row {
    display: flex;
    justify-content: space-between;
    font-size: 14px;
}

.success {
    color: #28a745;
}

.error {
    color: #dc3545;
}

/* Source Rules */
.source-rules {
    margin-top: 15px;
    padding-top: 15px;
    border-top: 1px solid #eee;
}

.source-rules h4 {
    font-size: 14px;
    margin-bottom: 10px;
    color: #666;
}

.rule-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 8px 0;
    border-bottom: 1px solid #f0f0f0;
}

.source-ip {
    font-family: monospace;
    font-size: 13px;
    color: #667eea;
}

.ratio {
    font-size: 12px;
    color: #666;
}

.add-rule {
    margin-top: 15px;
}

/* Sources Table */
.sources-table table {
    width: 100%;
    border-collapse: collapse;
}

.sources-table th,
.sources-table td {
    padding: 12px;
    text-align: left;
    border-bottom: 1px solid #eee;
}

.sources-table th {
    background: #f8f9fa;
    font-weight: 500;
    color: #666;
}

.sources-table td {
    font-family: monospace;
    font-size: 14px;
}

/* Configuration Info */
.config-info p {
    margin-bottom: 10px;
    font-size: 14px;
}

/* Modal Styles */
.modal {
    display: none;
    position: fixed;
    z-index: 1000;
    left: 0;
    top: 0;
    width: 100%;
    height: 100%;
    background-color: rgba(0, 0, 0, 0.5);
}

.modal-content {
    background-color: white;
    margin: 10% auto;
    padding: 30px;
    border-radius: 10px;
    width: 90%;
    max-width: 500px;
    position: relative;
}

.close {
    position: absolute;
    right: 15px;
    top: 15px;
    font-size: 24px;
    font-weight: bold;
    cursor: pointer;
    color: #aaa;
}

.close:hover {
    color: #000;
}

.form-actions {
    display: flex;
    gap: 10px;
    justify-content: flex-end;
    margin-top: 20px;
}

/* Info Box */
.info {
    margin-top: 20px;
    padding: 15px;
    background: #f8f9fa;
    border-radius: 5px;
    font-size: 12px;
    color: #666;
}

/* Responsive Design */
@media (max-width: 768px) {
    .container {
        padding: 20px 10px;
    }
    
    .navbar {
        padding: 15px 20px;
        flex-direction: column;
        gap: 15px;
    }
    
    .nav-controls {
        flex-direction: column;
        gap: 10px;
    }
    
    .stats-grid {
        grid-template-columns: 1fr 1fr;
    }
    
    .lb-grid {
        grid-template-columns: 1fr;
    }
    
    .stat-row {
        flex-direction: column;
        gap: 5px;
    }
    
    .rule-item {
        flex-direction: column;
        align-items: flex-start;
        gap: 5px;
    }
}
`
}

/*
Get JavaScript functions
*/
func getJavaScript() string {
	return `
// Modal functionality
function showAddRuleModal(lbAddress) {
    document.getElementById('modalLBAddress').value = lbAddress;
    document.getElementById('addRuleModal').style.display = 'block';
}

function closeModal() {
    document.getElementById('addRuleModal').style.display = 'none';
    document.getElementById('addRuleForm').reset();
}

// Close modal when clicking outside
window.onclick = function(event) {
    const modal = document.getElementById('addRuleModal');
    if (event.target === modal) {
        closeModal();
    }
}

// Toggle load balancer
async function toggleLoadBalancer(lbAddress, enabled) {
    try {
        const response = await fetch('/api/lb/toggle', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({
                lb_address: lbAddress,
                enabled: enabled
            })
        });
        
        const result = await response.json();
        if (!result.success) {
            alert('Failed to toggle load balancer');
            // Revert checkbox state
            event.target.checked = !enabled;
        } else {
            // Success - page will refresh automatically
            setTimeout(() => location.reload(), 500);
        }
    } catch (error) {
        alert('Error toggling load balancer: ' + error.message);
        // Revert checkbox state
        event.target.checked = !enabled;
    }
}

// Add source IP rule
document.getElementById('addRuleForm').addEventListener('submit', async function(e) {
    e.preventDefault();
    
    const formData = new FormData(e.target);
    const ruleData = {
        lb_address: formData.get('lb_address'),
        source_ip: formData.get('source_ip'),
        contention_ratio: parseInt(formData.get('contention_ratio')),
        description: formData.get('description')
    };
    
    try {
        const response = await fetch('/api/rules', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(ruleData)
        });
        
        const result = await response.json();
        if (result.success) {
            closeModal();
            location.reload();
        } else {
            alert('Failed to add rule');
        }
    } catch (error) {
        alert('Error adding rule: ' + error.message);
    }
});

// Remove source IP rule
async function removeRule(lbAddress, sourceIP) {
    if (!confirm('Are you sure you want to remove this rule?')) {
        return;
    }
    
    try {
        const response = await fetch('/api/rules?' + new URLSearchParams({
            lb_address: lbAddress,
            source_ip: sourceIP
        }), {
            method: 'DELETE'
        });
        
        const result = await response.json();
        if (result.success) {
            location.reload();
        } else {
            alert('Failed to remove rule');
        }
    } catch (error) {
        alert('Error removing rule: ' + error.message);
    }
}

// Auto-refresh functionality
let autoRefreshInterval;

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

// Initialize auto-refresh when page loads
document.addEventListener('DOMContentLoaded', function() {
    startAutoRefresh();
});

// Stop auto-refresh when page is hidden
document.addEventListener('visibilitychange', function() {
    if (document.visibilityState === 'hidden') {
        stopAutoRefresh();
    } else {
        startAutoRefresh();
    }
});
`
} 