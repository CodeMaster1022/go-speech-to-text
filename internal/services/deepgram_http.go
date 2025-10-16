package services

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// DeepgramHTTPService handles Deepgram API interactions using HTTP client
type DeepgramHTTPService struct {
	apiKey string
	client *http.Client
	logger *logrus.Logger
}

// TranscriptionResult represents the result of a transcription
type TranscriptionResult struct {
	Text       string  `json:"text"`
	Confidence float64 `json:"confidence"`
	Words      []Word  `json:"words,omitempty"`
}

// Word represents a single word in the transcription
type Word struct {
	Word       string  `json:"word"`
	Confidence float64 `json:"confidence"`
	Start      float64 `json:"start"`
	End        float64 `json:"end"`
}

// DeepgramResponse represents the response from Deepgram API
type DeepgramResponse struct {
	Results struct {
		Channels []struct {
			Alternatives []struct {
				Transcript string  `json:"transcript"`
				Confidence float64 `json:"confidence"`
				Words      []struct {
					Word       string  `json:"word"`
					Confidence float64 `json:"confidence"`
					Start      float64 `json:"start"`
					End        float64 `json:"end"`
				} `json:"words,omitempty"`
			} `json:"alternatives"`
		} `json:"channels"`
	} `json:"results"`
}

// NewDeepgramHTTPService creates a new Deepgram HTTP service instance
func NewDeepgramHTTPService(apiKey string, logger *logrus.Logger) (*DeepgramHTTPService, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("Deepgram API key is required")
	}

	return &DeepgramHTTPService{
		apiKey: apiKey,
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		logger: logger,
	}, nil
}

// TranscribeAudio transcribes audio from a reader and returns the result
func (ds *DeepgramHTTPService) TranscribeAudio(ctx context.Context, audioReader io.Reader, options *TranscriptionOptions) (*TranscriptionResult, error) {
	// Create a buffer to hold the multipart form data
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add the audio file
	part, err := writer.CreateFormFile("audio", "audio.wav")
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}

	// Copy audio data to the form
	if _, err := io.Copy(part, audioReader); err != nil {
		return nil, fmt.Errorf("failed to copy audio data: %w", err)
	}

	// Add transcription options
	if options.Language != "" {
		writer.WriteField("language", options.Language)
	}
	if options.Punctuate {
		writer.WriteField("punctuate", "true")
	}
	if options.Diarize {
		writer.WriteField("diarize", "true")
	}
	if options.SmartFormat {
		writer.WriteField("smart_format", "true")
	}
	if options.MedicalVocab {
		writer.WriteField("medical_vocab", "true")
	}
	if options.Redact {
		writer.WriteField("redact", "true")
	}
	if options.Multichannel {
		writer.WriteField("multichannel", "true")
	}
	if options.Alternatives > 0 {
		writer.WriteField("alternatives", fmt.Sprintf("%d", options.Alternatives))
	}
	if options.ProfanityFilter {
		writer.WriteField("profanity_filter", "true")
	}

	// Close the writer
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("failed to close multipart writer: %w", err)
	}

	// Create the HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.deepgram.com/v1/listen", &body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Token "+ds.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Send the request
	resp, err := ds.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse the response
	var deepgramResp DeepgramResponse
	if err := json.NewDecoder(resp.Body).Decode(&deepgramResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Extract results
	if len(deepgramResp.Results.Channels) == 0 {
		return nil, fmt.Errorf("no transcription results received")
	}

	channel := deepgramResp.Results.Channels[0]
	if len(channel.Alternatives) == 0 {
		return nil, fmt.Errorf("no transcription alternatives received")
	}

	alternative := channel.Alternatives[0]

	// Build word list if requested
	var words []Word
	if options.IncludeWords && len(alternative.Words) > 0 {
		for _, word := range alternative.Words {
			words = append(words, Word{
				Word:       word.Word,
				Confidence: word.Confidence,
				Start:      word.Start,
				End:        word.End,
			})
		}
	}

	result := &TranscriptionResult{
		Text:       alternative.Transcript,
		Confidence: alternative.Confidence,
		Words:      words,
	}

	ds.logger.WithFields(logrus.Fields{
		"confidence":  result.Confidence,
		"word_count":  len(words),
		"text_length": len(result.Text),
	}).Info("Transcription completed successfully")

	return result, nil
}

// TranscriptionOptions holds options for transcription
type TranscriptionOptions struct {
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
}

// DefaultTranscriptionOptions returns default transcription options
func DefaultTranscriptionOptions() *TranscriptionOptions {
	return &TranscriptionOptions{
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
	}
}
