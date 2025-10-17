package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// DeepgramStreamingService handles real-time streaming to Deepgram
type DeepgramStreamingService struct {
	apiKey string
	client *http.Client
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
		client: &http.Client{
			Timeout: 0, // No timeout for streaming
		},
		logger: logger,
	}, nil
}

// StartStreamingTranscription starts a streaming transcription session
func (ds *DeepgramStreamingService) StartStreamingTranscription(
	ctx context.Context,
	options *StreamingTranscriptionOptions,
	audioChunks <-chan []byte,
	results chan<- *TranscriptionData,
) error {
	// Build query parameters
	params := ds.buildStreamingParams(options)
	url := fmt.Sprintf("https://api.deepgram.com/v1/listen?%s", params)

	// Create a pipe to send audio chunks - MUST be created BEFORE the request
	reader, writer := io.Pipe()

	// Create HTTP request with the pipe reader as the body
	req, err := http.NewRequestWithContext(ctx, "POST", url, reader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Token "+ds.apiKey)
	req.Header.Set("Content-Type", "audio/raw")
	req.Header.Set("Transfer-Encoding", "chunked")

	ds.logger.Info("Starting Deepgram streaming connection...")

	// Start goroutine to send audio chunks to the pipe
	go ds.sendAudioChunks(ctx, writer, audioChunks)

	// Make the HTTP request in a goroutine to handle response streaming
	go func() {
		resp, err := ds.client.Do(req)
		if err != nil {
			ds.logger.WithError(err).Error("Failed to connect to Deepgram")
			close(results)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			ds.logger.Errorf("Deepgram returned status %d: %s", resp.StatusCode, string(bodyBytes))
			close(results)
			return
		}

		ds.logger.Info("Deepgram streaming connection established successfully")

		// Read responses (this blocks until the connection closes)
		ds.readResponses(resp.Body, results)
	}()

	return nil
}

// sendAudioChunks sends audio chunks to Deepgram
func (ds *DeepgramStreamingService) sendAudioChunks(ctx context.Context, writer io.Writer, audioChunks <-chan []byte) {
	defer func() {
		if closer, ok := writer.(io.Closer); ok {
			closer.Close()
		}
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
			
			n, err := writer.Write(chunk)
			if err != nil {
				ds.logger.WithError(err).Error("Failed to send audio chunk to Deepgram")
				return
			}
			
			chunkCount++
			totalBytes += n
			
			if chunkCount == 1 {
				ds.logger.Info("First audio chunk sent to Deepgram successfully")
			}
			
			if chunkCount%50 == 0 {
				ds.logger.Infof("Sent %d audio chunks to Deepgram (%d bytes total)", chunkCount, totalBytes)
			}
		case <-ctx.Done():
			ds.logger.Infof("Context cancelled, stopping audio sender after %d chunks (%d bytes)", chunkCount, totalBytes)
			return
		}
	}
}

// readResponses reads responses from Deepgram
func (ds *DeepgramStreamingService) readResponses(reader io.Reader, results chan<- *TranscriptionData) {
	defer close(results)

	decoder := json.NewDecoder(reader)
	for {
		var rawResponse map[string]interface{}
		if err := decoder.Decode(&rawResponse); err != nil {
			if err == io.EOF {
				break
			}
			ds.logger.WithError(err).Error("Failed to decode streaming response")
			continue
		}

		// Log the raw response for debugging (only first 500 chars to avoid spam)
		responseStr := fmt.Sprintf("%+v", rawResponse)
		if len(responseStr) > 500 {
			ds.logger.Debugf("Raw Deepgram response (truncated): %s...", responseStr[:500])
		} else {
			ds.logger.Debugf("Raw Deepgram response: %s", responseStr)
		}

		// Parse the actual Deepgram response format
		// Deepgram sends: {"results": {"channels": [{"alternatives": [{"transcript": "...", "confidence": 0.9}]}]}}
		if resultsMap, ok := rawResponse["results"].(map[string]interface{}); ok {
			if channels, ok := resultsMap["channels"].([]interface{}); ok && len(channels) > 0 {
				if channel, ok := channels[0].(map[string]interface{}); ok {
					if alternatives, ok := channel["alternatives"].([]interface{}); ok && len(alternatives) > 0 {
						if alt, ok := alternatives[0].(map[string]interface{}); ok {
							transcript := ""
							confidence := 0.0
							isFinal := true

							if t, ok := alt["transcript"].(string); ok {
								transcript = t
							}
							if c, ok := alt["confidence"].(float64); ok {
								confidence = c
							}
							if speechFinal, ok := rawResponse["speech_final"].(bool); ok {
								isFinal = speechFinal
							} else if isFinalField, ok := rawResponse["is_final"].(bool); ok {
								isFinal = isFinalField
							}

							// Skip empty transcripts
							if transcript == "" {
								ds.logger.Debug("Skipping empty transcript")
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

							ds.logger.Infof("✅ Parsed transcript: '%s', confidence: %.2f, is_final: %v", 
								transcript, confidence, isFinal)

							select {
							case results <- transcriptionData:
							case <-time.After(5 * time.Second):
								ds.logger.Warn("Timeout sending transcription result")
							}
						}
					}
				}
			}
		}
	}
}

// buildStreamingParams builds query parameters for streaming
func (ds *DeepgramStreamingService) buildStreamingParams(options *StreamingTranscriptionOptions) string {
	var params []string

	// Set encoding to linear16 (raw PCM)
	params = append(params, "encoding=linear16")
	
	// Set sample rate for linear16
	params = append(params, "sample_rate=16000")
	
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
		InterimResults:  true,
		Endpointing:     true,
		VadEvents:       false,
	}
}
