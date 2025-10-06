package main

import (
	"context"
	"syscall"
	//"encoding/json"
	"flag"
	//"fmt"
	//"html/template"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"

	//"strconv"
	//"strings"
	"sync"
	//"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// Config holds application configuration
type Config struct {
	ModemWSURL     string
	WebPort        string
	ReconnectDelay time.Duration
	RequestTimeout time.Duration
	PingInterval   time.Duration
	MaxReconnect   int
	LogLevel       string
	BufferSize     int
}

// ModemStatus holds parsed modem status information
type ModemStatus struct {
	mu              sync.RWMutex
	LastUpdate      time.Time        `json:"last_update"`
	RSSI            int              `json:"rssi"`
	NetworkType     string           `json:"network_type"`
	SignalStrength  int              `json:"signal_strength"`
	SignalQuality   int              `json:"signal_quality"`
	RSRQ            int              `json:"rsrq"`
	RSRP            int              `json:"rsrp"`
	DataFlow        []DataFlowRecord `json:"data_flow"`
	ConnectionStats ConnectionStats  `json:"connection_stats"`
	IsConnected     bool             `json:"is_connected"`
}

// DataFlowRecord holds data flow information
type DataFlowRecord struct {
	Timestamp time.Time `json:"timestamp"`
	ReportID  string    `json:"report_id"`
	ULBytes   int64     `json:"ul_bytes"`
	DLBytes   int64     `json:"dl_bytes"`
	ULRate    int64     `json:"ul_rate"`
	DLRate    int64     `json:"dl_rate"`
	TotalUL   int64     `json:"total_ul"`
	TotalDL   int64     `json:"total_dl"`
}

// ConnectionStats holds connection statistics
type ConnectionStats struct {
	TotalReconnects  int64         `json:"total_reconnects"`
	LastDisconnect   time.Time     `json:"last_disconnect"`
	Uptime           time.Duration `json:"uptime"`
	BytesReceived    int64         `json:"bytes_received"`
	MessagesReceived int64         `json:"messages_received"`
}

// WebSocketClient manages WebSocket connection
type WebSocketClient struct {
	config         *Config
	modemStatus    *ModemStatus
	conn           *websocket.Conn
	shutdown       chan struct{}
	reconnect      chan struct{}
	stats          *ConnectionStats
	logger         *log.Logger
	reconnectCount int
}

// Server manages HTTP server and WebSocket client
type Server struct {
	config      *Config
	modemStatus *ModemStatus
	wsClient    *WebSocketClient
	mux         *http.ServeMux
	logger      *log.Logger
}

// Regular expressions for parsing modem data
var (
	rssiRegex   = regexp.MustCompile(`\^RSSI:(-?\d+)`)
	hcsqRegex   = regexp.MustCompile(`\^HCSQ:"([^"]+)",(\d+),(\d+),(\d+),(\d+)`)
	dsflowRegex = regexp.MustCompile(`\^DSFLOWRPT:([^,]+),([^,]+),([^,]+),([^,]+),([^,]+),([^,]+),([^,]+)`)
)

func main() {
	config := parseFlags()

	// Setup logging
	logger := setupLogger(config.LogLevel)

	// Create modem status
	modemStatus := &ModemStatus{
		DataFlow: make([]DataFlowRecord, 0),
	}

	// Create and start server
	server := NewServer(config, modemStatus, logger)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	setupSignalHandler(cancel, logger)

	// Start the server
	if err := server.Start(ctx); err != nil {
		logger.Fatalf("Failed to start server: %v", err)
	}

	logger.Println("Server shutdown complete")
}

func parseFlags() *Config {
	config := &Config{}

	flag.StringVar(&config.ModemWSURL, "modem-ws-url", "ws://localhost:8080/modem",
		"Modem WebSocket URL")
	flag.StringVar(&config.WebPort, "web-port", "8080",
		"Web dashboard port")
	flag.DurationVar(&config.ReconnectDelay, "reconnect-delay", 5*time.Second,
		"WebSocket reconnect delay")
	flag.DurationVar(&config.RequestTimeout, "request-timeout", 10*time.Second,
		"HTTP request timeout")
	flag.DurationVar(&config.PingInterval, "ping-interval", 30*time.Second,
		"WebSocket ping interval")
	flag.IntVar(&config.MaxReconnect, "max-reconnect", 10,
		"Maximum reconnection attempts (0 = infinite)")
	flag.StringVar(&config.LogLevel, "log-level", "info",
		"Log level (debug, info, warn, error)")
	flag.IntVar(&config.BufferSize, "buffer-size", 100,
		"WebSocket message buffer size")

	flag.Parse()

	return config
}

func setupLogger(level string) *log.Logger {
	logger := log.New(os.Stdout, "", log.LstdFlags|log.Lmicroseconds)
	return logger
}

func setupSignalHandler(cancel context.CancelFunc, logger *log.Logger) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Printf("Received signal: %v. Shutting down gracefully...", sig)
		cancel()

		// Force exit after timeout
		time.Sleep(10 * time.Second)
		logger.Println("Forcing shutdown due to timeout")
		os.Exit(1)
	}()
}
