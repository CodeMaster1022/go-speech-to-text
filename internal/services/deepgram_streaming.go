package services

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

// DeepgramStreamingService handles real-time streaming to Deepgram via WebSocket
type DeepgramStreamingService struct {
	apiKey string
	logger *logrus.Logger
}

// StreamingTranscriptionOptions holds options for streaming transcription
type StreamingTranscriptionOptions struct {
	Model           string
	Language        string
	Punctuate       bool
	Diarize         bool
	SmartFormat     bool
	IncludeWords    bool
	MedicalVocab    bool
	Redact          bool
	Multichannel    bool
	Alternatives    int
	ProfanityFilter bool
	InterimResults  bool
	Endpointing     bool
	VadEvents       bool
}

// StreamingResponse represents a streaming response from Deepgram
type StreamingResponse struct {
	Type string `json:"type"`
	Data struct {
		Transcript string  `json:"transcript"`
		Confidence float64 `json:"confidence"`
		Words      []struct {
			Word       string  `json:"word"`
			Confidence float64 `json:"confidence"`
			Start      float64 `json:"start"`
			End        float64 `json:"end"`
		} `json:"words,omitempty"`
		IsFinal bool `json:"is_final"`
	} `json:"data"`
}

// NewDeepgramStreamingService creates a new Deepgram streaming service
func NewDeepgramStreamingService(apiKey string, logger *logrus.Logger) (*DeepgramStreamingService, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Deepgram API key is required")
	}

	return &DeepgramStreamingService{
		apiKey: apiKey,
		logger: logger,
	}, nil
}

// StartStreamingTranscription starts a streaming transcription session via WebSocket
func (ds *DeepgramStreamingService) StartStreamingTranscription(
	ctx context.Context,
	options *StreamingTranscriptionOptions,
	audioChunks <-chan []byte,
	results chan<- *TranscriptionData,
) error {
	// Build WebSocket URL with query parameters
	params := ds.buildStreamingParams(options)
	wsURL := fmt.Sprintf("wss://api.deepgram.com/v1/listen?%s", params)

	ds.logger.Infof("Connecting to Deepgram WebSocket: %s", wsURL)

	// Parse URL
	u, err := url.Parse(wsURL)
	if err != nil {
		return fmt.Errorf("failed to parse WebSocket URL: %w", err)
	}

	// Set up WebSocket dialer with authorization header
	header := make(map[string][]string)
	header["Authorization"] = []string{"Token " + ds.apiKey}

	// Connect to Deepgram WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), header)
	if err != nil {
		ds.logger.WithError(err).Error("Failed to connect to Deepgram WebSocket")
		return fmt.Errorf("failed to connect to Deepgram WebSocket: %w", err)
	}

	ds.logger.Info("✅ Connected to Deepgram WebSocket successfully")

	// Start goroutines to send audio and receive responses
	go ds.sendAudioChunksWS(ctx, conn, audioChunks)
	go ds.readResponsesWS(ctx, conn, results)

	return nil
}

// sendAudioChunksWS sends audio chunks to Deepgram via WebSocket
func (ds *DeepgramStreamingService) sendAudioChunksWS(ctx context.Context, conn *websocket.Conn, audioChunks <-chan []byte) {
	defer func() {
		// Send close message to Deepgram
		conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		ds.logger.Info("Audio chunk sender stopped")
	}()

	chunkCount := 0
	totalBytes := 0
	
	for {
		select {
		case chunk, ok := <-audioChunks:
			if !ok {
				ds.logger.Infof("Audio chunks channel closed after sending %d chunks (%d bytes total)", chunkCount, totalBytes)
				return
			}
			
			// Send audio as binary WebSocket message
			if err := conn.WriteMessage(websocket.BinaryMessage, chunk); err != nil {
				ds.logger.WithError(err).Error("Failed to send audio chunk to Deepgram WebSocket")
				return
			}
			
			chunkCount++
			totalBytes += len(chunk)
			
			if chunkCount == 1 {
				ds.logger.Info("✅ First audio chunk sent to Deepgram WebSocket successfully")
			}
			
			if chunkCount%50 == 0 {
				ds.logger.Infof("Sent %d audio chunks to Deepgram (%d bytes total)", chunkCount, totalBytes)
			}
		case <-ctx.Done():
			// ds.logger.Infof("Context cancelled, stopping audio sender after %d chunks (%d bytes)", chunkCount, totalBytes)
			return
		}
	}
}

// readResponsesWS reads responses from Deepgram WebSocket
func (ds *DeepgramStreamingService) readResponsesWS(ctx context.Context, conn *websocket.Conn, results chan<- *TranscriptionData) {
	defer func() {
		ds.logger.Info("Closing Deepgram WebSocket response reader")
		conn.Close()
		close(results)
	}()

	ds.logger.Info("Starting to read Deepgram WebSocket responses...")
	
	responseCount := 0
	
	for {
		select {
		case <-ctx.Done():
			ds.logger.Info("Context cancelled, stopping WebSocket reader")
			return
		default:
			// Read message from WebSocket
			_, message, err := conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					ds.logger.WithError(err).Error("WebSocket closed unexpectedly")
				} else {
					// ds.logger.Infof("Deepgram WebSocket closed (received %d responses)", responseCount)
				}
				return
			}

			// Parse JSON response
			var rawResponse map[string]interface{}
			if err := json.Unmarshal(message, &rawResponse); err != nil {
				ds.logger.WithError(err).Error("Failed to parse WebSocket message")
				continue
			}

			responseCount++
			// ds.logger.Infof("📨 Received response #%d from Deepgram", responseCount)

			// Log the raw response as JSON for better debugging
			if jsonBytes, err := json.Marshal(rawResponse); err == nil {
				responseStr := string(jsonBytes)
				if len(responseStr) > 1000 {
					ds.logger.Infof("Raw response (truncated): %s...", responseStr[:1000])
				} else {
					ds.logger.Infof("Raw response: %s", responseStr)
				}
			}

			// Parse the WebSocket response format
			// WebSocket sends: {"channel": {"alternatives": [{"transcript": "...", "confidence": 0.9}]}, "is_final": false}
			channel, hasChannel := rawResponse["channel"].(map[string]interface{})
			if !hasChannel {
				ds.logger.Debug("Response missing 'channel' field (might be metadata)")
				continue
			}

			alternatives, hasAlternatives := channel["alternatives"].([]interface{})
			if !hasAlternatives || len(alternatives) == 0 {
				ds.logger.Warn("Channel missing 'alternatives' field or empty")
				continue
			}

			alt, isAltMap := alternatives[0].(map[string]interface{})
			if !isAltMap {
				ds.logger.Warn("Alternative is not a map")
				continue
			}
			transcript := ""
			confidence := 0.0
			isFinal := true

			if t, ok := alt["transcript"].(string); ok {
				transcript = t
			}
			if c, ok := alt["confidence"].(float64); ok {
				confidence = c
			}
			
			// Check for speech_final in the top-level response
			if speechFinal, ok := rawResponse["speech_final"].(bool); ok {
				isFinal = speechFinal
			} else if isFinalField, ok := rawResponse["is_final"].(bool); ok {
				isFinal = isFinalField
			} else {
				// Default to treating as interim if no finality flag
				isFinal = false
			}

			// ds.logger.Infof("🔍 Extracted: transcript='%s', confidence=%.2f, speech_final=%v", 
				// transcript, confidence, isFinal)

			// Skip empty transcripts
			if transcript == "" {
				// ds.logger.Info("⚠️ Skipping empty transcript")
				continue
			}

			transcriptionData := &TranscriptionData{
				Text:       transcript,
				Confidence: confidence,
				IsPartial:  !isFinal,
			}

			// Parse words if available
			if words, ok := alt["words"].([]interface{}); ok && len(words) > 0 {
				wordList := make([]Word, len(words))
				for i, w := range words {
					if wordMap, ok := w.(map[string]interface{}); ok {
						word := Word{}
						if wrd, ok := wordMap["word"].(string); ok {
							word.Word = wrd
						}
						if conf, ok := wordMap["confidence"].(float64); ok {
							word.Confidence = conf
						}
						if start, ok := wordMap["start"].(float64); ok {
							word.Start = start
						}
						if end, ok := wordMap["end"].(float64); ok {
							word.End = end
						}
						wordList[i] = word
					}
				}
				transcriptionData.Words = wordList
			}

			// ds.logger.Infof("✅ Parsed transcript: '%s', confidence: %.2f, is_final: %v", 
				// transcript, confidence, isFinal)

			// Try to send the result
			select {
			case results <- transcriptionData:
				ds.logger.Info("✅ Transcription sent to results channel")
			case <-time.After(5 * time.Second):
				ds.logger.Error("❌ Timeout sending transcription result")
			}
		}
	}
}


// buildStreamingParams builds query parameters for streaming
func (ds *DeepgramStreamingService) buildStreamingParams(options *StreamingTranscriptionOptions) string {
	var params []string

	// Set encoding to linear16 (raw PCM)
	params = append(params, "encoding=linear16")
	
	// Set sample rate for linear16 (48000 is common for browsers)
	params = append(params, "sample_rate=48000")
	
	// Set channels (mono)
	params = append(params, "channels=1")

	if options.Model != "" {
		params = append(params, fmt.Sprintf("model=%s", options.Model))
	}
	if options.Language != "" {
		params = append(params, fmt.Sprintf("language=%s", options.Language))
	}
	if options.Punctuate {
		params = append(params, "punctuate=true")
	}
	if options.Diarize {
		params = append(params, "diarize=true")
	}
	if options.SmartFormat {
		params = append(params, "smart_format=true")
	}
	if options.MedicalVocab {
		params = append(params, "medical_vocab=true")
	}
	if options.Redact {
		params = append(params, "redact=true")
	}
	if options.Multichannel {
		params = append(params, "multichannel=true")
	}
	if options.Alternatives > 0 {
		params = append(params, fmt.Sprintf("alternatives=%d", options.Alternatives))
	}
	if options.ProfanityFilter {
		params = append(params, "profanity_filter=true")
	}
	if options.InterimResults {
		params = append(params, "interim_results=true")
	}
	if options.Endpointing {
		params = append(params, "endpointing=true")
	}
	if options.VadEvents {
		params = append(params, "vad_events=true")
	}

	// Join parameters
	var result string
	for i, param := range params {
		if i > 0 {
			result += "&"
		}
		result += param
	}

	return result
}

// DefaultStreamingOptions returns default streaming options
func DefaultStreamingOptions() *StreamingTranscriptionOptions {
	return &StreamingTranscriptionOptions{
		Model:           "nova-2",
		Language:        "en",
		Punctuate:       true,
		Diarize:         false,
		SmartFormat:     true,
		IncludeWords:    true,
		MedicalVocab:    true,
		Redact:          false,
		Multichannel:    false,
		Alternatives:    1,
		ProfanityFilter: false,
		InterimResults:  true,  // Send partial results as user speaks
		Endpointing:     false, // Don't wait for pauses - send immediately
		VadEvents:       true,  // Voice activity detection for better real-time
	}
}
