package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

func NewWebSocketClient(config *Config, modemStatus *ModemStatus, logger *log.Logger) *WebSocketClient {
	return &WebSocketClient{
		config:      config,
		modemStatus: modemStatus,
		shutdown:    make(chan struct{}),
		reconnect:   make(chan struct{}, 1),
		stats:       &ConnectionStats{},
		logger:      logger,
	}
}

func (w *WebSocketClient) Start(ctx context.Context) {
	w.logger.Println("Starting WebSocket client")

	for {
		select {
		case <-ctx.Done():
			w.logger.Println("WebSocket client stopping due to context cancellation")
			return
		case <-w.shutdown:
			w.logger.Println("WebSocket client stopping")
			return
		default:
			if err := w.connectAndListen(ctx); err != nil {
				w.logger.Printf("WebSocket connection error: %v", err)
			}

			// Check if we should reconnect
			if w.config.MaxReconnect > 0 && w.reconnectCount >= w.config.MaxReconnect {
				w.logger.Printf("Max reconnection attempts (%d) reached", w.config.MaxReconnect)
				return
			}

			w.logger.Printf("Reconnecting in %v...", w.config.ReconnectDelay)
			time.Sleep(w.config.ReconnectDelay)
			w.reconnectCount++
		}
	}
}

func (w *WebSocketClient) connectAndListen(ctx context.Context) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: w.config.RequestTimeout,
	}

	conn, _, err := dialer.Dial(w.config.ModemWSURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()

	w.conn = conn
	w.reconnectCount = 0
	atomic.AddInt64(&w.stats.TotalReconnects, 1)

	w.logger.Println("Successfully connected to modem WebSocket")

	// Update modem status
	w.modemStatus.mu.Lock()
	w.modemStatus.IsConnected = true
	w.modemStatus.LastUpdate = time.Now()
	w.modemStatus.mu.Unlock()

	// Start ping goroutine
	go w.pingHandler(ctx)

	// Listen for messages
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-w.shutdown:
			return nil
		default:
			conn.SetReadDeadline(time.Now().Add(w.config.PingInterval * 2))
			messageType, message, err := conn.ReadMessage()
			if err != nil {
				w.handleDisconnect()
				return fmt.Errorf("WebSocket read error: %v", err)
			}
			//log.Printf("message: %s", message)
			if messageType == websocket.TextMessage {
				w.handleMessage(message)
			}
		}
	}
}

func (w *WebSocketClient) handleMessage(message []byte) {
	atomic.AddInt64(&w.stats.BytesReceived, int64(len(message)))
	atomic.AddInt64(&w.stats.MessagesReceived, 1)

	messageStr := string(message)
	w.logger.Printf("DEBUG: Received message: %s", messageStr)

	// Parse different message types
	if strings.HasPrefix(messageStr, "^RSSI:") {
		w.parseRSSI(messageStr)
	} else if strings.HasPrefix(messageStr, "^HCSQ:") {
		w.parseHCSQ(messageStr)
	} else if strings.HasPrefix(messageStr, "^DSFLOWRPT:") {
		w.parseDSFLOW(messageStr)
	}
}

func (w *WebSocketClient) parseRSSI(data string) {
	w.logger.Printf("DEBUG.parseRSSI string:%s", data)
	matches := rssiRegex.FindStringSubmatch(data)
	if len(matches) == 2 {
		rssi, err := strconv.Atoi(matches[1])
		if err == nil {
			w.modemStatus.mu.Lock()
			w.modemStatus.RSSI = rssi
			w.modemStatus.LastUpdate = time.Now()
			w.modemStatus.mu.Unlock()
			w.logger.Printf("INFO: RSSI updated: %d", rssi)
		}
	}
}

func (w *WebSocketClient) parseHCSQ(data string) {
	matches := hcsqRegex.FindStringSubmatch(data)
	if len(matches) == 6 {
		networkType := matches[1]
		signalStrength, _ := strconv.Atoi(matches[2])
		signalQuality, _ := strconv.Atoi(matches[3])
		rsrq, _ := strconv.Atoi(matches[4])
		rsrp, _ := strconv.Atoi(matches[5])

		w.modemStatus.mu.Lock()
		w.modemStatus.NetworkType = networkType
		w.modemStatus.SignalStrength = signalStrength
		w.modemStatus.SignalQuality = signalQuality
		w.modemStatus.RSRQ = rsrq
		w.modemStatus.RSRP = rsrp
		w.modemStatus.LastUpdate = time.Now()
		w.modemStatus.mu.Unlock()

		w.logger.Printf("INFO: Network updated: %s, Strength: %d, Quality: %d, RSRQ: %d, RSRP: %d",
			networkType, signalStrength, signalQuality, rsrq, rsrp)
	}
}

func (w *WebSocketClient) parseDSFLOW(data string) {
	matches := dsflowRegex.FindStringSubmatch(data)
	if len(matches) == 8 {
		reportID := matches[1]
		ulBytes, _ := strconv.ParseInt(matches[2], 16, 64)
		dlBytes, _ := strconv.ParseInt(matches[3], 16, 64)
		totalUL, _ := strconv.ParseInt(matches[4], 16, 64)
		totalDL, _ := strconv.ParseInt(matches[5], 16, 64)

		record := DataFlowRecord{
			Timestamp: time.Now(),
			ReportID:  reportID,
			ULBytes:   ulBytes,
			DLBytes:   dlBytes,
			ULRate:    ulBytes, // This would need calculation based on time
			DLRate:    dlBytes, // This would need calculation based on time
			TotalUL:   totalUL,
			TotalDL:   totalDL,
		}

		w.modemStatus.mu.Lock()
		// Keep only last 100 records
		if len(w.modemStatus.DataFlow) >= 100 {
			w.modemStatus.DataFlow = w.modemStatus.DataFlow[1:]
		}
		w.modemStatus.DataFlow = append(w.modemStatus.DataFlow, record)
		w.modemStatus.LastUpdate = time.Now()
		w.modemStatus.mu.Unlock()

		w.logger.Printf("DEBUG: Data flow - UL: %d, DL: %d, Total UL: %d, Total DL: %d",
			ulBytes, dlBytes, totalUL, totalDL)
	}
}

func (w *WebSocketClient) pingHandler(ctx context.Context) {
	ticker := time.NewTicker(w.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.shutdown:
			return
		case <-ticker.C:
			if w.conn != nil {
				err := w.conn.WriteMessage(websocket.PingMessage, nil)
				if err != nil {
					w.logger.Printf("Ping error: %v", err)
					w.handleDisconnect()
					return
				}
			}
		}
	}
}

func (w *WebSocketClient) handleDisconnect() {
	w.modemStatus.mu.Lock()
	w.modemStatus.IsConnected = false
	w.modemStatus.mu.Unlock()

	w.stats.LastDisconnect = time.Now()
	w.logger.Println("WARN: Disconnected from modem WebSocket")
}

func (w *WebSocketClient) Stop() {
	close(w.shutdown)
	if w.conn != nil {
		w.conn.Close()
	}
}
