package main

import (
	"bufio"
	"context"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// MonitorStdin monitors the stdin stream
func (sa *SignalAnalyzer) MonitorStdin() {
	log.Println("Starting stdin monitoring...")

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		sa.processLine(line, "stdin")
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading from stdin: %v", err)
	}
}

// MonitorTTYD monitors ttyd WebSocket stream
func (sa *SignalAnalyzer) MonitorTTYD(ttydURL string) {
	log.Printf("Starting ttyd monitoring: %s", ttydURL)

	for {
		func() {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			conn, _, err := websocket.DefaultDialer.DialContext(ctx, ttydURL, nil)
			if err != nil {
				log.Printf("Failed to connect to ttyd: %v, retrying in 5 seconds...", err)
				time.Sleep(5 * time.Second)
				return
			}
			defer conn.Close()

			log.Printf("Connected to ttyd: %s", ttydURL)

			// Read messages from ttyd
			for {
				select {
				case <-ctx.Done():
					return
				default:
					_, message, err := conn.ReadMessage()
					if err != nil {
						log.Printf("Error reading from ttyd: %v, reconnecting...", err)
						return
					}

					line := string(message)
					sa.processLine(line, "ttyd")
				}
			}
		}()

		time.Sleep(5 * time.Second)
	}
}

// processLine processes a single line of modem output
func (sa *SignalAnalyzer) processLine(line string, source string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}

	log.Printf("[%s] Raw input: %s", source, line)

	status, dataUsage := sa.ParseModemOutput(line, source)

	if status != nil {
		log.Printf("[%s] Signal Status: %s - RSRP: %.1f dBm, SINR: %.1f dB, Quality: %s",
			source, status.SystemMode, status.RSRP_dBm, status.SINR_dB, status.SignalQuality)

		// Broadcast status update
		sa.Broadcast(map[string]interface{}{
			"type": "status",
			"data": status,
		})
	}

	if dataUsage != nil {
		log.Printf("[%s] Data Usage: UL: %d bytes, DL: %d bytes, Total UL: %d, Total DL: %d",
			source, dataUsage.UplinkBytes, dataUsage.DownlinkBytes,
			dataUsage.TotalUplink, dataUsage.TotalDownlink)

		// Broadcast data usage update
		sa.Broadcast(map[string]interface{}{
			"type": "data_usage",
			"data": dataUsage,
		})
	}
}

// ParseModemOutput parses the raw modem output and extracts signal metrics
func (sa *SignalAnalyzer) ParseModemOutput(line string, source string) (*ModemStatus, *DataUsage) {
	line = strings.TrimSpace(line)

	// Parse RSSI line
	if matches := rssiRegex.FindStringSubmatch(line); len(matches) == 2 {
		rssi, _ := strconv.Atoi(matches[1])
		rssi_dBm := convertCSQToDBm(rssi)

		sa.mu.Lock()
		defer sa.mu.Unlock()

		if sa.currentStatus == nil {
			sa.currentStatus = &ModemStatus{}
		}
		sa.currentStatus.Timestamp = time.Now()
		sa.currentStatus.RSSI = rssi
		sa.currentStatus.RSSI_dBm = rssi_dBm
		sa.currentStatus.Source = source

		return sa.currentStatus, nil
	}

	// Parse HCSQ line (LTE parameters)
	if matches := hcsqRegex.FindStringSubmatch(line); len(matches) == 6 {
		systemMode := matches[1]
		rsrp, _ := strconv.Atoi(matches[2])
		sinr, _ := strconv.Atoi(matches[3])
		rsrq, _ := strconv.Atoi(matches[4])
		// matches[5] is the 5th parameter, not typically used

		rsrp_dBm, sinr_dB, rsrq_dB := convertLTEValues(rsrp, sinr, rsrq)

		status := &ModemStatus{
			Timestamp:  time.Now(),
			SystemMode: systemMode,
			RSSI:       -1, // Will be filled from RSSI line
			RSRP:       rsrp,
			RSRP_dBm:   rsrp_dBm,
			SINR:       sinr,
			SINR_dB:    sinr_dB,
			RSRQ:       rsrq,
			RSRQ_dB:    rsrq_dB,
			Source:     source,
		}

		// Analyze signal quality
		status.SignalQuality = sa.analyzeSignalQuality(status)
		status.HealthStatus = sa.analyzeHealthStatus(status)

		sa.mu.Lock()
		defer sa.mu.Unlock()

		// Merge with existing RSSI data if available
		if sa.currentStatus != nil && sa.currentStatus.RSSI != -1 {
			status.RSSI = sa.currentStatus.RSSI
			status.RSSI_dBm = sa.currentStatus.RSSI_dBm
		}

		sa.currentStatus = status
		sa.statusHistory = append(sa.statusHistory, *status)

		// Keep history within bounds
		if len(sa.statusHistory) > sa.maxHistory {
			sa.statusHistory = sa.statusHistory[1:]
		}

		return status, nil
	}

	// Parse DSFLOWRPT line (data usage)
	if matches := dsflowRegex.FindStringSubmatch(line); len(matches) == 8 {
		reportID := matches[1]
		uplinkBytes, _ := strconv.ParseInt(matches[2], 16, 64)
		downlinkBytes, _ := strconv.ParseInt(matches[3], 16, 64)
		totalUplink, _ := strconv.ParseInt(matches[4], 16, 64)
		totalDownlink, _ := strconv.ParseInt(matches[5], 16, 64)

		dataUsage := &DataUsage{
			Timestamp:     time.Now(),
			ReportID:      reportID,
			UplinkBytes:   uplinkBytes,
			DownlinkBytes: downlinkBytes,
			TotalUplink:   totalUplink,
			TotalDownlink: totalDownlink,
		}

		sa.mu.Lock()
		defer sa.mu.Unlock()

		sa.currentDataUsage = dataUsage
		sa.dataUsageHistory = append(sa.dataUsageHistory, *dataUsage)

		// Keep data history within bounds
		if len(sa.dataUsageHistory) > sa.maxHistory/10 {
			sa.dataUsageHistory = sa.dataUsageHistory[1:]
		}

		return sa.currentStatus, dataUsage
	}

	return nil, nil
}
