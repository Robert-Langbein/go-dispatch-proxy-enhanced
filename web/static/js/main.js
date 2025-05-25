// Main JavaScript - Common functionality

// Modal functionality
function showAddRuleModal(lbAddress) {
    document.getElementById('modalLBAddress').value = lbAddress;
    document.getElementById('addRuleModal').style.display = 'block';
}

function showWeightModal(sourceIP, lbAddress) {
    document.getElementById('weightSourceIP').value = sourceIP;
    document.getElementById('weightLBAddress').value = lbAddress;
    document.getElementById('weightSourceIPDisplay').textContent = sourceIP;
    document.getElementById('weightLBDisplay').textContent = lbAddress;
    document.getElementById('weightModal').style.display = 'block';
}

function closeModal(modalId) {
    if (modalId) {
        document.getElementById(modalId).style.display = 'none';
        if (modalId === 'addRuleModal') {
            document.getElementById('addRuleForm').reset();
        } else if (modalId === 'weightModal') {
            document.getElementById('weightForm').reset();
        }
    } else {
        // Legacy support - close all modals
        document.getElementById('addRuleModal').style.display = 'none';
        document.getElementById('weightModal').style.display = 'none';
        document.getElementById('addRuleForm').reset();
        document.getElementById('weightForm').reset();
    }
}

// Close modal when clicking outside
window.onclick = function(event) {
    const addRuleModal = document.getElementById('addRuleModal');
    const weightModal = document.getElementById('weightModal');
    
    if (event.target === addRuleModal) {
        closeModal('addRuleModal');
    } else if (event.target === weightModal) {
        closeModal('weightModal');
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

// Utility functions
function formatBytes(bytes) {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}

function formatDuration(startTime) {
    const now = new Date();
    const diff = Math.floor((now - startTime) / 1000);
    
    if (diff < 60) return diff + 's';
    if (diff < 3600) return Math.floor(diff / 60) + 'm ' + (diff % 60) + 's';
    return Math.floor(diff / 3600) + 'h ' + Math.floor((diff % 3600) / 60) + 'm';
}

// Form handlers
document.addEventListener('DOMContentLoaded', function() {
    // Add source IP rule form handler
    const addRuleForm = document.getElementById('addRuleForm');
    if (addRuleForm) {
        addRuleForm.addEventListener('submit', async function(e) {
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
                    closeModal('addRuleModal');
                    location.reload();
                } else {
                    alert('Failed to add rule');
                }
            } catch (error) {
                alert('Error adding rule: ' + error.message);
            }
        });
    }

    // Weight form handler
    const weightForm = document.getElementById('weightForm');
    if (weightForm) {
        weightForm.addEventListener('submit', async function(e) {
            e.preventDefault();
            
            const formData = new FormData(e.target);
            const weightData = {
                source_ip: formData.get('source_ip'),
                lb_address: formData.get('lb_address'),
                contention_ratio: parseInt(formData.get('contention_ratio')),
                description: formData.get('description')
            };
            
            try {
                const response = await fetch('/api/connection/weight', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify(weightData)
                });
                
                const result = await response.json();
                if (result.success) {
                    closeModal('weightModal');
                    location.reload();
                } else {
                    alert('Failed to set connection weight');
                }
            } catch (error) {
                alert('Error setting connection weight: ' + error.message);
            }
        });
    }
}); 