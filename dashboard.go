package main

const dashboardHTML = `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Modem Status Dashboard</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { 
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; 
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            padding: 20px;
        }
        .container { 
            max-width: 1200px; 
            margin: 0 auto; 
        }
        .header { 
            text-align: center; 
            color: white;
            margin-bottom: 30px;
        }
        .header h1 { 
            font-size: 2.5rem; 
            margin-bottom: 10px;
            text-shadow: 2px 2px 4px rgba(0,0,0,0.3);
        }
        .header p { 
            font-size: 1.1rem; 
            opacity: 0.9;
        }
        .dashboard {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 20px;
            margin-bottom: 20px;
        }
        .card {
            background: white;
            border-radius: 15px;
            padding: 25px;
            box-shadow: 0 10px 30px rgba(0,0,0,0.2);
            transition: transform 0.3s ease;
        }
        .card:hover {
            transform: translateY(-5px);
        }
        .card h2 {
            color: #333;
            margin-bottom: 15px;
            font-size: 1.3rem;
            border-bottom: 2px solid #667eea;
            padding-bottom: 10px;
        }
        .status-item {
            display: flex;
            justify-content: space-between;
            margin-bottom: 10px;
            padding: 8px 0;
            border-bottom: 1px solid #f0f0f0;
        }
        .status-label {
            font-weight: 600;
            color: #555;
        }
        .status-value {
            font-weight: 700;
            color: #333;
        }
        .connected { color: #28a745; }
        .disconnected { color: #dc3545; }
        .signal-excellent { color: #28a745; }
        .signal-good { color: #17a2b8; }
        .signal-fair { color: #ffc107; }
        .signal-poor { color: #dc3545; }
        .chart-container {
            height: 200px;
            margin-top: 15px;
        }
        .last-update {
            text-align: center;
            color: #666;
            font-style: italic;
            margin-top: 10px;
        }
    </style>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>📡 Modem Status Dashboard</h1>
            <p>Real-time monitoring of modem connection and data flow</p>
        </div>
        
        <div class="dashboard">
            <!-- Connection Status Card -->
            <div class="card">
                <h2>🔗 Connection Status</h2>
                <div id="connection-status">
                    <div class="status-item">
                        <span class="status-label">Status:</span>
                        <span class="status-value" id="status">Loading...</span>
                    </div>
                    <div class="status-item">
                        <span class="status-label">Uptime:</span>
                        <span class="status-value" id="uptime">--</span>
                    </div>
                    <div class="status-item">
                        <span class="status-label">Last Update:</span>
                        <span class="status-value" id="last-update">--</span>
                    </div>
                    <div class="status-item">
                        <span class="status-label">Reconnects:</span>
                        <span class="status-value" id="reconnects">--</span>
                    </div>
                </div>
            </div>

            <!-- Signal Quality Card -->
            <div class="card">
                <h2>📶 Signal Quality</h2>
                <div id="signal-quality">
                    <div class="status-item">
                        <span class="status-label">Network Type:</span>
                        <span class="status-value" id="network-type">--</span>
                    </div>
                    <div class="status-item">
                        <span class="status-label">RSSI:</span>
                        <span class="status-value" id="rssi">--</span>
                    </div>
                    <div class="status-item">
                        <span class="status-label">Signal Strength:</span>
                        <span class="status-value" id="signal-strength">--</span>
                    </div>
                    <div class="status-item">
                        <span class="status-label">Signal Quality:</span>
                        <span class="status-value" id="signal-quality-value">--</span>
                    </div>
                    <div class="status-item">
                        <span class="status-label">RSRQ:</span>
                        <span class="status-value" id="rsrq">--</span>
                    </div>
                    <div class="status-item">
                        <span class="status-label">RSRP:</span>
                        <span class="status-value" id="rsrp">--</span>
                    </div>
                </div>
            </div>

            <!-- Data Usage Card -->
            <div class="card">
                <h2>📊 Data Flow</h2>
                <div id="data-flow">
                    <div class="status-item">
                        <span class="status-label">Upload Total:</span>
                        <span class="status-value" id="total-ul">--</span>
                    </div>
                    <div class="status-item">
                        <span class="status-label">Download Total:</span>
                        <span class="status-value" id="total-dl">--</span>
                    </div>
                    <div class="status-item">
                        <span class="status-label">Last UL Rate:</span>
                        <span class="status-value" id="ul-rate">--</span>
                    </div>
                    <div class="status-item">
                        <span class="status-label">Last DL Rate:</span>
                        <span class="status-value" id="dl-rate">--</span>
                    </div>
                </div>
                <div class="chart-container">
                    <canvas id="dataFlowChart"></canvas>
                </div>
            </div>
        </div>

        <div class="last-update">
            Last updated: <span id="update-time">--</span>
        </div>
    </div>

    <script>
        let dataFlowChart = null;
        
        function updateDashboard() {
            fetch('/api/status')
                .then(response => response.json())
                .then(data => {
                    // Update connection status
                    document.getElementById('status').textContent = 
                        data.is_connected ? '🟢 Connected' : '🔴 Disconnected';
                    document.getElementById('status').className = 
                        data.is_connected ? 'status-value connected' : 'status-value disconnected';
                    
                    document.getElementById('uptime').textContent = 
                        formatDuration(data.connection_stats.uptime);
                    document.getElementById('last-update').textContent = 
                        new Date(data.last_update).toLocaleTimeString();
                    document.getElementById('reconnects').textContent = 
                        data.connection_stats.total_reconnects;

                    // Update signal quality
                    document.getElementById('network-type').textContent = data.network_type || '--';
                    document.getElementById('rssi').textContent = data.rssi !== 0 ? data.rssi + ' dBm' : '--';
                    document.getElementById('signal-strength').textContent = 
                        data.signal_strength !== 0 ? data.signal_strength + '%' : '--';
                    document.getElementById('signal-quality-value').textContent = 
                        data.signal_quality !== 0 ? data.signal_quality + '%' : '--';
                    document.getElementById('rsrq').textContent = data.rsrq !== 0 ? data.rsrq : '--';
                    document.getElementById('rsrp').textContent = data.rsrp !== 0 ? data.rsrp + ' dBm' : '--';

                    // Update data flow
                    if (data.data_flow && data.data_flow.length > 0) {
                        const latest = data.data_flow[data.data_flow.length - 1];
                        document.getElementById('total-ul').textContent = formatBytes(latest.total_ul);
                        document.getElementById('total-dl').textContent = formatBytes(latest.total_dl);
                        document.getElementById('ul-rate').textContent = formatBytes(latest.ul_rate) + '/s';
                        document.getElementById('dl-rate').textContent = formatBytes(latest.dl_rate) + '/s';
                        
                        updateChart(data.data_flow);
                    }

                    document.getElementById('update-time').textContent = new Date().toLocaleTimeString();
                })
                .catch(error => {
                    console.error('Error fetching status:', error);
                    document.getElementById('status').textContent = '🔴 Error';
                    document.getElementById('status').className = 'status-value disconnected';
                });
        }

        function updateChart(flowData) {
            const ctx = document.getElementById('dataFlowChart').getContext('2d');
            const labels = flowData.slice(-20).map((_, index) => ` + "`" + `T-${19-index}` + "`" + `);
            const ulData = flowData.slice(-20).map(d => d.ul_bytes);
            const dlData = flowData.slice(-20).map(d => d.dl_bytes);

            if (dataFlowChart) {
                dataFlowChart.destroy();
            }

            dataFlowChart = new Chart(ctx, {
                type: 'line',
                data: {
                    labels: labels,
                    datasets: [
                        {
                            label: 'Upload',
                            data: ulData,
                            borderColor: 'rgb(255, 99, 132)',
                            backgroundColor: 'rgba(255, 99, 132, 0.1)',
                            tension: 0.4
                        },
                        {
                            label: 'Download',
                            data: dlData,
                            borderColor: 'rgb(54, 162, 235)',
                            backgroundColor: 'rgba(54, 162, 235, 0.1)',
                            tension: 0.4
                        }
                    ]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    scales: {
                        y: {
                            beginAtZero: true,
                            title: {
                                display: true,
                                text: 'Bytes'
                            }
                        }
                    }
                }
            });
        }

        function formatDuration(duration) {
            if (!duration) return '--';
            const seconds = Math.floor(duration / 1000);
            const hours = Math.floor(seconds / 3600);
            const minutes = Math.floor((seconds % 3600) / 60);
            const secs = seconds % 60;
            return ` + "`" + `${hours.toString().padStart(2, '0')}:${minutes.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}` + "`" + `;
        }

        function formatBytes(bytes) {
            if (bytes === 0) return '0 B';
            const k = 1024;
            const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
            const i = Math.floor(Math.log(bytes) / Math.log(k));
            return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
        }

        // Update dashboard every 2 seconds
        setInterval(updateDashboard, 2000);
        updateDashboard(); // Initial update
    </script>
</body>
</html>
`
