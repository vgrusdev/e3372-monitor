package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"text/template"
	"time"
)

func NewServer(config *Config, modemStatus *ModemStatus, logger *log.Logger) *Server {
	return &Server{
		config:      config,
		modemStatus: modemStatus,
		mux:         http.NewServeMux(),
		logger:      logger,
	}
}

func (s *Server) Start(ctx context.Context) error {
	s.logger.Printf("Starting modem monitoring server on port %s", s.config.WebPort)
	s.logger.Printf("Modem WebSocket URL: %s", s.config.ModemWSURL)

	// Create WebSocket client
	s.wsClient = NewWebSocketClient(s.config, s.modemStatus, s.logger)

	// Start WebSocket client
	go s.wsClient.Start(ctx)

	// Setup HTTP routes
	s.setupRoutes()

	// Create HTTP server
	server := &http.Server{
		Addr:         ":" + s.config.WebPort,
		Handler:      s.mux,
		ReadTimeout:  s.config.RequestTimeout,
		WriteTimeout: s.config.RequestTimeout,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		s.logger.Printf("HTTP server listening on http://localhost:%s", s.config.WebPort)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		s.logger.Println("Shutdown signal received, closing server...")
	case err := <-serverErr:
		s.logger.Printf("Server error: %v", err)
	}

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	// Stop WebSocket client
	s.wsClient.Stop()

	// Shutdown HTTP server
	if err := server.Shutdown(shutdownCtx); err != nil {
		s.logger.Printf("HTTP server shutdown error: %v", err)
		return err
	}

	return nil
}

func (s *Server) setupRoutes() {
	// API endpoints
	s.mux.HandleFunc("/api/status", s.handleStatusAPI)
	s.mux.HandleFunc("/api/stats", s.handleStatsAPI)
	s.mux.HandleFunc("/api/flow", s.handleFlowAPI)
	s.mux.HandleFunc("/api/health", s.handleHealthAPI)

	// Web dashboard
	s.mux.HandleFunc("/", s.handleDashboard)

	// Static files
	s.mux.Handle("/static/", http.StripPrefix("/static/",
		http.FileServer(http.Dir("./static"))))
}

// HTTP Handlers
func (s *Server) handleStatusAPI(w http.ResponseWriter, r *http.Request) {
	s.modemStatus.mu.RLock()
	defer s.modemStatus.mu.RUnlock()

	// Calculate uptime
	uptime := time.Since(s.wsClient.stats.LastDisconnect)
	if s.modemStatus.IsConnected {
		s.modemStatus.ConnectionStats.Uptime = uptime
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.modemStatus)
}

func (s *Server) handleStatsAPI(w http.ResponseWriter, r *http.Request) {
	stats := struct {
		ConnectionStats ConnectionStats `json:"connection_stats"`
		Config          Config          `json:"config"`
	}{
		ConnectionStats: *s.wsClient.stats,
		Config:          *s.config,
	}

	// Update uptime
	if s.modemStatus.IsConnected {
		stats.ConnectionStats.Uptime = time.Since(s.wsClient.stats.LastDisconnect)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (s *Server) handleFlowAPI(w http.ResponseWriter, r *http.Request) {
	s.modemStatus.mu.RLock()
	defer s.modemStatus.mu.RUnlock()

	// Return only data flow records
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(s.modemStatus.DataFlow)
}

func (s *Server) handleHealthAPI(w http.ResponseWriter, r *http.Request) {
	health := struct {
		Status      string    `json:"status"`
		LastUpdate  time.Time `json:"last_update"`
		IsConnected bool      `json:"is_connected"`
		Uptime      string    `json:"uptime"`
	}{
		Status:      "healthy",
		LastUpdate:  s.modemStatus.LastUpdate,
		IsConnected: s.modemStatus.IsConnected,
		Uptime:      time.Since(s.wsClient.stats.LastDisconnect).String(),
	}

	if !s.modemStatus.IsConnected {
		health.Status = "disconnected"
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	tmpl := template.Must(template.New("dashboard").Parse(dashboardHTML))

	data := struct {
		Config Config
	}{
		Config: *s.config,
	}

	w.Header().Set("Content-Type", "text/html")
	tmpl.Execute(w, data)
}
