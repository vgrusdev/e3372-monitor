package main

import (
	//"bufio"
	//"context"
	//"encoding/json"
	//"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	//"strconv"
	//"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// StartAPIServer starts the HTTP API server
func (sa *SignalAnalyzer) StartAPIServer(addr string) {
	mux := http.NewServeMux()
	mux.HandleFunc("/status", sa.handleStatus)
	mux.HandleFunc("/data-usage", sa.handleDataUsage)
	mux.HandleFunc("/history", sa.handleHistory)
	mux.HandleFunc("/current", sa.handleCurrent)
	mux.HandleFunc("/ws", sa.handleWebSocket)
	
	// Serve a simple HTML dashboard
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		html := `<!DOCTYPE html>
<html>
<head>
    <title>LTE Modem Monitor</title>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body { 
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif; 
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
            min-height: 100vh;
            padding: 20px;
            color: #333;
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
        }
        .header {
            background: rgba(255, 255, 255, 0.95);
            padding: 2rem;
            border-radius: 15px;
            margin-bottom: 20px;
            box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
            backdrop-filter: blur(10px);
        }
        .header h1 {
            color: #2c3e50;
            margin-bottom: 0.5rem;
            font-size: 2.5rem;
        }
        .header p {
            color: #7f8c8d;
            font-size: 1.1rem;
        }
        .dashboard {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(350px, 1fr));
            gap: 20px;
            margin-bottom: 20px;
        }
        .card {
            background: rgba(255, 255, 255, 0.95);
            padding: 1.5rem;
            border-radius: 15px;
            box-shadow: 0 8px 32px rgba(0, 0, 0, 0.1);
            backdrop-filter: blur(10px);
            border-left: 5px solid;
        }
        .card h3 {
            color: #2c3e50;
            margin-bottom: 1rem;
            font-size: 1.3rem;
            display: flex;
            align-items: center;
            gap: 10px;
        }
        .card h3 i {
            font-size: 1.2em;
        }
        .status-item {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 0.8rem 0;
            border-bottom: 1px solid #ecf0f1;
        }
        .status-item:last-child {
            border-bottom: none;
        }
        .status-label {
            font-weight: 600;
            color: #34495e;
        }
        .status-value {
            font-weight: 700;
        }
        .quality-excellent { border-left-color: #27ae60; }
        .quality-good { border-left-color: #3498db; }
        .quality-fair { border-left-color: #f39c12; }
        .quality-poor { border-left-color: #e74c3c; }
        .health-optimal { border-left-color: #27ae60; }
        .health-stable { border-left-color: #3498db; }
        .health-degraded { border-left-color: #f39c12; }
        .health-critical { border-left-color: #e74c3c; }
        .data-card { border-left-color: #9b59b6; }
        .source-badge {
            background: #34495e;
            color: white;
            padding: 0.3rem 0.8rem;
            border-radius: 20px;
            font-size: 0.8rem;
            font-weight: 600;
        }
        .timestamp {
            color: #7f8c8d;
            font-size: 0.9rem;
            text-align: right;
            margin-top: 1rem;
        }
        .no-data {
            text-align: center;
            color: #7f8c8d;
            font-style: italic;
            padding: 2rem;
        }
        .connection-status {
            display: inline-block;
            width: 10px;
            height: 10px;
            border-radius: 50%;
            margin-right: 8px;
        }
        .connected { background: #27ae60; }
        .disconnected { background: #e74c3c; }
        @keyframes pulse {
            0% { opacity: 1; }
            50% { opacity: 0.5; }
            100% { opacity: 1; }
        }
        .updating {
            animation: pulse 1.5s infinite;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>📶 LTE Modem Monitor</h1>
            <p>Real-time signal quality and data usage monitoring</p>
        </div>
        
        <div class="dashboard">
            <div class="card" id="signalCard">
                <h3><span class="connection-status" id="wsStatus"></span> Signal Status</h3>
                <div id="signalStatus" class="no-data">Waiting for data...</div>
            </div>
            
            <div class="card" id="dataCard">
                <h3>📊 Data Usage</h3>
                <div id="dataUsage" class="no-data">Waiting for data...</div>
            </div>
        </div>
    </div>

    <script>
        let ws;
        let reconnectTimeout;
        
        function connectWebSocket() {
            const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
            const wsUrl = protocol + '//' + window.location.host + '/ws';
            
            ws = new WebSocket(wsUrl);
            
            ws.onopen = function() {
                console.log('WebSocket connected');
                updateConnectionStatus(true);
                clearTimeout(reconnectTimeout);
            };
            
            ws.onmessage = function(event) {
                const data = JSON.parse(event.data);
                updateDashboard(data);
            };
            
            ws.onclose = function() {
                console.log('WebSocket disconnected');
                updateConnectionStatus(false);
                // Attempt reconnect after 3 seconds
                reconnectTimeout = setTimeout(connectWebSocket, 3000);
            };
            
            ws.onerror = function(error) {
                console.error('WebSocket error:', error);
            };
        }
        
        function updateConnectionStatus(connected) {
            const statusElement = document.getElementById('wsStatus');
            statusElement.className = 'connection-status ' + (connected ? 'connected' : 'disconnected');
            statusElement.title = connected ? 'Connected' : 'Disconnected';
        }
        
        function updateDashboard(data) {
            if (data.type === 'status') {
                updateSignalStatus(data.data);
            } else if (data.type === 'data_usage') {
                updateDataUsage(data.data);
            }
        }
        
        function updateSignalStatus(status) {
            const container = document.getElementById('signalStatus');
            const card = document.getElementById('signalCard');
            
            // Update card border color based on quality
            card.className = 'card quality-' + status.signal_quality.toLowerCase();
            
            container.innerHTML = \`
                <div class="status-item">
                    <span class="status-label">System Mode:</span>
                    <span class="status-value">\${status.system_mode || 'N/A'}</span>
                </div>
                <div class="status-item">
                    <span class="status-label">RSRP:</span>
                    <span class="status-value">\${status.rsrp_dbm ? status.rsrp_dbm.toFixed(1) + ' dBm' : 'N/A'}</span>
                </div>
                <div class="status-item">
                    <span class="status-label">SINR:</span>
                    <span class="status-value">\${status.sinr_db ? status.sinr_db.toFixed(1) + ' dB' : 'N/A'}</span>
                </div>
                <div class="status-item">
                    <span class="status-label">RSSI:</span>
                    <span class="status-value">\${status.rssi_dbm ? status.rssi_dbm.toFixed(1) + ' dBm' : 'N/A'}</span>
                </div>
                <div class="status-item">
                    <span class="status-label">RSRQ:</span>
                    <span class="status-value">\${status.rsrq_db ? status.rsrq_db.toFixed(1) + ' dB' : 'N/A'}</span>
                </div>
                <div class="status-item">
                    <span class="status-label">Signal Quality:</span>
                    <span class="status-value" style="color: \${getQualityColor(status.signal_quality)}">\${status.signal_quality}</span>
                </div>
                <div class="status-item">
                    <span class="status-label">Health Status:</span>
                    <span class="status-value" style="color: \${getHealthColor(status.health_status)}">\${status.health_status}</span>
                </div>
                <div class="status-item">
                    <span class="status-label">Source:</span>
                    <span class="source-badge">\${status.source}</span>
                </div>
                <div class="timestamp">
                    Last updated: \${new Date(status.timestamp).toLocaleString()}
                </div>
            \`;
        }
        
        function updateDataUsage(data) {
            const container = document.getElementById('dataUsage');
            
            if (!data) {
                container.innerHTML = '<div class="no-data">No data usage information available</div>';
                return;
            }
            
            function formatBytes(bytes) {
                if (bytes === 0) return '0 B';
                const k = 1024;
                const sizes = ['B', 'KB', 'MB', 'GB'];
                const i = Math.floor(Math.log(bytes) / Math.log(k));
                return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
            }
            
            container.innerHTML = \`
                <div class="status-item">
                    <span class="status-label">Uplink Rate:</span>
                    <span class="status-value">\${formatBytes(data.uplink_bytes)}/s</span>
                </div>
                <div class="status-item">
                    <span class="status-label">Downlink Rate:</span>
                    <span class="status-value">\${formatBytes(data.downlink_bytes)}/s</span>
                </div>
                <div class="status-item">
                    <span class="status-label">Total Uplink:</span>
                    <span class="status-value">\${formatBytes(data.total_uplink)}</span>
                </div>
                <div class="status-item">
                    <span class="status-label">Total Downlink:</span>
                    <span class="status-value">\${formatBytes(data.total_downlink)}</span>
                </div>
                <div class="status-item">
                    <span class="status-label">Report ID:</span>
                    <span class="status-value">\${data.report_id}</span>
                </div>
                <div class="timestamp">
                    Last updated: \${new Date(data.timestamp).toLocaleString()}
                </div>
            \`;
        }
        
        function getQualityColor(quality) {
            const colors = {
                'Excellent': '#27ae60',
                'Good': '#3498db', 
                'Fair': '#f39c12',
                'Poor': '#e74c3c'
            };
            return colors[quality] || '#7f8c8d';
        }
        
        function getHealthColor(health) {
            const colors = {
                'Optimal': '#27ae60',
                'Stable': '#3498db',
                'Degraded': '#f39c12',
                'Critical': '#e74c3c'
            };
            return colors[health] || '#7f8c8d';
        }
        
        // Initialize
        connectWebSocket();
        
        // Fetch initial data
        fetch('/current')
            .then(response => response.json())
            .then(data => {
                if (data.status) {
                    updateSignalStatus(data.status);
                }
                if (data.data_usage) {
                    updateDataUsage(data.data_usage);
                }
            })
            .catch(error => {
                console.error('Error fetching initial data:', error);
            });
    </script>
</body>
</html>`
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html))
	})
	
	log.Printf("Starting API server on %s", addr)
	go func() {
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Fatalf("API server failed: %v", err)
		}
	}()
}

// WebSocket handler for real-time updates
func (sa *SignalAnalyzer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	
	// Register client
	sa.mu.Lock()
	sa.clients[conn] = true
	sa.mu.Unlock()
	
	log.Printf("WebSocket client connected (total: %d)", len(sa.clients))
	
	// Send current status immediately
	if status := sa.GetCurrentStatus(); status != nil {
		conn.WriteJSON(map[string]interface{}{
			"type": "status",
			"data": status,
		})
	}
	if dataUsage := sa.GetCurrentDataUsage(); dataUsage != nil {
		conn.WriteJSON(map[string]interface{}{
			"type": "data_usage", 
			"data": dataUsage,
		})
	}
	
	// Handle messages and keep connection alive
	defer func() {
		sa.mu.Lock()
		delete(sa.clients, conn)
		sa.mu.Unlock()
		conn.Close()
		log.Printf("WebSocket client disconnected (total: %d)", len(sa.clients))
	}()
	
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// HTTP handler for current status
func (sa *SignalAnalyzer) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := sa.GetCurrentStatus()
	if status == nil {
		http.Error(w, `{"error": "No status data available"}`, http.StatusNotFound)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// HTTP handler for current data usage
func (sa *SignalAnalyzer) handleDataUsage(w http.ResponseWriter, r *http.Request) {
	dataUsage := sa.GetCurrentDataUsage()
	if dataUsage == nil {
		http.Error(w, `{"error": "No data usage data available"}`, http.StatusNotFound)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dataUsage)
}

// HTTP handler for status history
func (sa *SignalAnalyzer) handleHistory(w http.ResponseWriter, r *http.Request) {
	history := sa.GetHistory()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(history)
}

// HTTP handler for combined current data
func (sa *SignalAnalyzer) handleCurrent(w http.ResponseWriter, r *http.Request) {
	status := sa.GetCurrentStatus()
	dataUsage := sa.GetCurrentDataUsage()
	
	response := map[string]interface{}{
		"status":     status,
		"data_usage": dataUsage,
		"timestamp":  time.Now(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
// GetCurrentStatus returns the current modem status
func (sa *SignalAnalyzer) GetCurrentStatus() *ModemStatus {
	sa.mu.RLock()
	defer sa.mu.RUnlock()
	return sa.currentStatus
}

// GetCurrentDataUsage returns the current data usage
func (sa *SignalAnalyzer) GetCurrentDataUsage() *DataUsage {
	sa.mu.RLock()
	defer sa.mu.RUnlock()
	return sa.currentDataUsage
}

// GetHistory returns status history
func (sa *SignalAnalyzer) GetHistory() []ModemStatus {
	sa.mu.RLock()
	defer sa.mu.RUnlock()
	return append([]ModemStatus{}, sa.statusHistory...)
}

// GetDataUsageHistory returns data usage history
func (sa *SignalAnalyzer) GetDataUsageHistory() []DataUsage {
	sa.mu.RLock()
	defer sa.mu.RUnlock()
	return append([]DataUsage{}, sa.dataUsageHistory...)
}