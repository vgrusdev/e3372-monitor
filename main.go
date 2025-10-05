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

// ModemStatus represents the parsed LTE modem status
type ModemStatus struct {
	Timestamp     time.Time  `json:"timestamp"`
	SystemMode    string     `json:"system_mode"`
	RSSI          int        `json:"rssi"`
	RSSI_dBm      float64    `json:"rssi_dbm"`
	RSRP          int        `json:"rsrp"`
	RSRP_dBm      float64    `json:"rsrp_dbm"`
	SINR          int        `json:"sinr"`
	SINR_dB       float64    `json:"sinr_db"`
	RSRQ          int        `json:"rsrq"`
	RSRQ_dB       float64    `json:"rsrq_db"`
	SignalQuality string     `json:"signal_quality"`
	HealthStatus  string     `json:"health_status"`
	DataUsage     *DataUsage `json:"data_usage,omitempty"`
	Source        string     `json:"source"` // "stdin" or "ttyd"
}

// DataUsage represents data flow information from DSFLOWRPT
type DataUsage struct {
	Timestamp     time.Time `json:"timestamp"`
	ReportID      string    `json:"report_id"`
	UplinkBytes   int64     `json:"uplink_bytes"`
	DownlinkBytes int64     `json:"downlink_bytes"`
	TotalUplink   int64     `json:"total_uplink"`
	TotalDownlink int64     `json:"total_downlink"`
}

// SignalAnalyzer analyzes signal quality based on metrics
type SignalAnalyzer struct {
	mu               sync.RWMutex
	currentStatus    *ModemStatus
	currentDataUsage *DataUsage
	statusHistory    []ModemStatus
	dataUsageHistory []DataUsage
	maxHistory       int
	clients          map[*websocket.Conn]bool
	broadcast        chan interface{}
}

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	// Regex patterns for parsing modem output
	rssiRegex   = regexp.MustCompile(`\^RSSI:(\d+)`)
	hcsqRegex   = regexp.MustCompile(`\^HCSQ:"([^"]+)",(\d+),(\d+),(\d+),(\d+)`)
	dsflowRegex = regexp.MustCompile(`\^DSFLOWRPT:([^,]+),([^,]+),([^,]+),([^,]+),([^,]+),([^,]+),([^,]+)`)
)

// NewSignalAnalyzer creates a new signal analyzer
func NewSignalAnalyzer(maxHistory int) *SignalAnalyzer {
	return &SignalAnalyzer{
		statusHistory:    make([]ModemStatus, 0, maxHistory),
		dataUsageHistory: make([]DataUsage, 0, maxHistory/10),
		maxHistory:       maxHistory,
		clients:          make(map[*websocket.Conn]bool),
		broadcast:        make(chan interface{}, 100),
	}
}

func main() {
	// Configuration
	apiAddr := ":8080"
	ttydURL := "ws://10.134.15.1:8080/ws" // Default ttyd URL
	maxHistory := 1000

	// Create signal analyzer
	analyzer := NewSignalAnalyzer(maxHistory)

	// Start broadcaster
	go analyzer.RunBroadcaster()

	// Start API server
	analyzer.StartAPIServer(apiAddr)

	// Start monitoring from both sources
	go analyzer.MonitorStdin()
	go analyzer.MonitorTTYD(ttydURL)

	log.Println("LTE Modem Monitor started")
	log.Printf("API available at http://localhost%s", apiAddr)
	log.Printf("Monitoring ttyd stream at: %s", ttydURL)
	log.Println("Monitoring stdin stream...")
	log.Println("Supported formats:")
	log.Println("  ^RSSI:31")
	log.Println("  ^HCSQ:\"LTE\",90,80,166,20")
	log.Println("  ^DSFLOWRPT:00002CB1,00000154,000000EB,00000000125DBB6B,000000000F51DB0A,00000000,00000000")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)
	<-sigChan

	log.Println("Shutting down...")
}
