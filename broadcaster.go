package main

import (
	"log"

	"github.com/gorilla/websocket"
)

// Broadcast sends message to all WebSocket clients
func (sa *SignalAnalyzer) Broadcast(message interface{}) {
	sa.broadcast <- message
}

// RunBroadcaster handles broadcasting to WebSocket clients
func (sa *SignalAnalyzer) RunBroadcaster() {
	for {
		message := <-sa.broadcast

		sa.mu.RLock()
		clients := make([]*websocket.Conn, 0, len(sa.clients))
		for client := range sa.clients {
			clients = append(clients, client)
		}
		sa.mu.RUnlock()

		for _, client := range clients {
			err := client.WriteJSON(message)
			if err != nil {
				log.Printf("WebSocket write error: %v", err)
				client.Close()
				sa.mu.Lock()
				delete(sa.clients, client)
				sa.mu.Unlock()
			}
		}
	}
}
