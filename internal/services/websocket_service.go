package services

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// WebSocketService handles WebSocket connections for real-time speech-to-text
type WebSocketService struct {
	upgrader websocket.Upgrader
	clients  map[string]*Client
	mutex    sync.RWMutex
	logger   *logrus.Logger
}

// Client represents a WebSocket client connection
type Client struct {
	ID           string
	Conn         *websocket.Conn
	Send         chan []byte
	Language     string
	SessionID    string
	IsRecording  bool
	LastActivity time.Time
}

// Message types for WebSocket communication
type WSMessage struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	SessionID string      `json:"session_id,omitempty"`
	Timestamp int64       `json:"timestamp"`
}

// TranscriptionData represents real-time transcription data
type TranscriptionData struct {
	Text       string    `json:"text"`
	Confidence float64   `json:"confidence"`
	Words      []Word    `json:"words,omitempty"`
	IsPartial  bool      `json:"is_partial"`
	Language   string    `json:"language"`
	SessionID  string    `json:"session_id"`
}

// MedicalTerm represents a medical term with confidence score
type MedicalTerm struct {
	Term       string  `json:"term"`
	Confidence float64 `json:"confidence"`
	Start      float64 `json:"start"`
	End        float64 `json:"end"`
	Category   string  `json:"category,omitempty"`
}

// NewWebSocketService creates a new WebSocket service
func NewWebSocketService(logger *logrus.Logger) *WebSocketService {
	return &WebSocketService{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for development
			},
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		clients: make(map[string]*Client),
		logger:  logger,
	}
}

// HandleWebSocket handles WebSocket connections
func (ws *WebSocketService) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := ws.upgrader.Upgrade(w, r, nil)
	if err != nil {
		ws.logger.WithError(err).Error("Failed to upgrade WebSocket connection")
		return
	}

	clientID := uuid.New().String()
	client := &Client{
		ID:           clientID,
		Conn:         conn,
		Send:         make(chan []byte, 256),
		Language:     "en", // Default language
		SessionID:    uuid.New().String(),
		IsRecording:  false,
		LastActivity: time.Now(),
	}

	ws.mutex.Lock()
	ws.clients[clientID] = client
	ws.mutex.Unlock()

	ws.logger.WithField("client_id", clientID).Info("New WebSocket client connected")

	// Start goroutines for reading and writing
	go ws.handleClient(client)
	go ws.writePump(client)
}

// handleClient handles messages from a specific client
func (ws *WebSocketService) handleClient(client *Client) {
	defer func() {
		ws.mutex.Lock()
		delete(ws.clients, client.ID)
		ws.mutex.Unlock()
		client.Conn.Close()
		ws.logger.WithField("client_id", client.ID).Info("WebSocket client disconnected")
	}()

	// Set read deadline
	client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	client.Conn.SetPongHandler(func(string) error {
		client.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		var msg WSMessage
		err := client.Conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				ws.logger.WithError(err).WithField("client_id", client.ID).Error("WebSocket error")
			}
			break
		}

		client.LastActivity = time.Now()
		ws.processMessage(client, &msg)
	}
}

// writePump handles writing messages to the client
func (ws *WebSocketService) writePump(client *Client) {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		client.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-client.Send:
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				client.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := client.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages
			n := len(client.Send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-client.Send)
			}

			if err := w.Close(); err != nil {
				return
			}
		case <-ticker.C:
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// processMessage processes incoming messages from clients
func (ws *WebSocketService) processMessage(client *Client, msg *WSMessage) {
	switch msg.Type {
	case "start_recording":
		client.IsRecording = true
		client.SessionID = uuid.New().String()
		ws.sendMessage(client, WSMessage{
			Type:      "recording_started",
			Data:      map[string]string{"session_id": client.SessionID},
			SessionID: client.SessionID,
			Timestamp: time.Now().Unix(),
		})
		ws.logger.WithField("client_id", client.ID).Info("Recording started")

	case "stop_recording":
		client.IsRecording = false
		ws.sendMessage(client, WSMessage{
			Type:      "recording_stopped",
			Data:      map[string]string{"session_id": client.SessionID},
			SessionID: client.SessionID,
			Timestamp: time.Now().Unix(),
		})
		ws.logger.WithField("client_id", client.ID).Info("Recording stopped")

	case "pause_recording":
		client.IsRecording = false
		ws.sendMessage(client, WSMessage{
			Type:      "recording_paused",
			Data:      map[string]string{"session_id": client.SessionID},
			SessionID: client.SessionID,
			Timestamp: time.Now().Unix(),
		})

	case "set_language":
		if lang, ok := msg.Data.(string); ok {
			client.Language = lang
			ws.sendMessage(client, WSMessage{
				Type:      "language_set",
				Data:      map[string]string{"language": lang},
				SessionID: client.SessionID,
				Timestamp: time.Now().Unix(),
			})
		}

	case "audio_chunk":
		// Process audio chunk for real-time transcription
		ws.processAudioChunk(client, msg)

	case "ping":
		ws.sendMessage(client, WSMessage{
			Type:      "pong",
			Data:      map[string]string{"status": "ok"},
			SessionID: client.SessionID,
			Timestamp: time.Now().Unix(),
		})
	}
}

// processAudioChunk processes audio data for real-time transcription
func (ws *WebSocketService) processAudioChunk(client *Client, msg *WSMessage) {
	// Extract audio data from message
	audioData, ok := msg.Data.([]interface{})
	if !ok {
		ws.logger.Error("Invalid audio data format")
		return
	}

	// Convert to byte array
	audioBytes := make([]byte, len(audioData))
	for i, v := range audioData {
		if val, ok := v.(float64); ok {
			audioBytes[i] = byte(val)
		}
	}

	// For now, we'll simulate transcription response
	// In a full implementation, you would:
	// 1. Accumulate audio chunks
	// 2. Send to Deepgram streaming API
	// 3. Process real-time responses
	
	// Simulate partial transcription
	if len(audioBytes) > 0 {
		transcriptionData := TranscriptionData{
			Text:       "Processing audio...",
			Confidence:  0.85,
			IsPartial:  true,
			Language:   client.Language,
			SessionID:  client.SessionID,
		}

		ws.sendMessage(client, WSMessage{
			Type:      "transcription_partial",
			Data:      transcriptionData,
			SessionID: client.SessionID,
			Timestamp: time.Now().Unix(),
		})
	}
}

// sendMessage sends a message to a specific client
func (ws *WebSocketService) sendMessage(client *Client, msg WSMessage) {
	select {
	case client.Send <- ws.marshalMessage(msg):
	default:
		close(client.Send)
		ws.mutex.Lock()
		delete(ws.clients, client.ID)
		ws.mutex.Unlock()
	}
}

// BroadcastMessage broadcasts a message to all connected clients
func (ws *WebSocketService) BroadcastMessage(msg WSMessage) {
	ws.mutex.RLock()
	defer ws.mutex.RUnlock()

	for _, client := range ws.clients {
		ws.sendMessage(client, msg)
	}
}

// GetConnectedClients returns the number of connected clients
func (ws *WebSocketService) GetConnectedClients() int {
	ws.mutex.RLock()
	defer ws.mutex.RUnlock()
	return len(ws.clients)
}

// marshalMessage marshals a message to JSON
func (ws *WebSocketService) marshalMessage(msg WSMessage) []byte {
	data, err := json.Marshal(msg)
	if err != nil {
		ws.logger.WithError(err).Error("Failed to marshal WebSocket message")
		return []byte("{}")
	}
	return data
}

// CleanupInactiveClients removes inactive clients
func (ws *WebSocketService) CleanupInactiveClients() {
	ws.mutex.Lock()
	defer ws.mutex.Unlock()

	now := time.Now()
	for clientID, client := range ws.clients {
		if now.Sub(client.LastActivity) > 5*time.Minute {
			client.Conn.Close()
			delete(ws.clients, clientID)
			ws.logger.WithField("client_id", clientID).Info("Removed inactive client")
		}
	}
}
