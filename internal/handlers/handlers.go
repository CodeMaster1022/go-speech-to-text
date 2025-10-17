package handlers

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"speech-to-text-backend/internal/config"
	"speech-to-text-backend/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// Handlers contains all HTTP handlers
type Handlers struct {
	deepgramService        *services.DeepgramHTTPService
	websocketService       *services.WebSocketService
	medicalVocabService    *services.MedicalVocabularyService
	config                 *config.Config
	logger                 *logrus.Logger
}

// New creates a new handlers instance
func New(cfg *config.Config, logger *logrus.Logger) *Handlers {
	// Initialize Deepgram service
	deepgramService, err := services.NewDeepgramHTTPService(cfg.DeepgramAPIKey, logger)
	if err != nil {
		logger.WithError(err).Fatal("Failed to initialize Deepgram service")
	}

	// Initialize WebSocket service
	websocketService := services.NewWebSocketService(logger, cfg.DeepgramAPIKey)

	// Initialize Medical Vocabulary service
	medicalVocabService := services.NewMedicalVocabularyService()

	return &Handlers{
		deepgramService:     deepgramService,
		websocketService:   websocketService,
		medicalVocabService: medicalVocabService,
		config:             cfg,
		logger:             logger,
	}
}

// TranscribeAudio handles audio file upload and transcription
func (h *Handlers) TranscribeAudio(c *gin.Context) {
	// Parse multipart form
	err := c.Request.ParseMultipartForm(h.config.MaxFileSize)
	if err != nil {
		h.logger.WithError(err).Error("Failed to parse multipart form")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Failed to parse form data",
		})
		return
	}

	// Get the audio file
	file, header, err := c.Request.FormFile("audio")
	if err != nil {
		h.logger.WithError(err).Error("Failed to get audio file")
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Audio file is required",
		})
		return
	}
	defer file.Close()

	// Validate file extension
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !h.isValidAudioFormat(ext) {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": fmt.Sprintf("Unsupported audio format: %s. Supported formats: %v", ext, h.config.AllowedFormats),
		})
		return
	}

	// Get transcription options from form
	options := h.getTranscriptionOptions(c)

	// Create context with timeout
	ctx := context.Background()

	// Transcribe audio
	result, err := h.deepgramService.TranscribeAudio(ctx, file, options)
	if err != nil {
		h.logger.WithError(err).Error("Transcription failed")
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Transcription failed",
		})
		return
	}

	// Return successful response
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"filename":    header.Filename,
			"text":        result.Text,
			"confidence":  result.Confidence,
			"words":       result.Words,
			"word_count":  len(result.Words),
		},
	})
}

// GetSupportedFormats returns the list of supported audio formats
func (h *Handlers) GetSupportedFormats(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"supported_formats": h.config.AllowedFormats,
			"max_file_size_mb":  h.config.MaxFileSize / (1024 * 1024),
		},
	})
}

// isValidAudioFormat checks if the file extension is supported
func (h *Handlers) isValidAudioFormat(ext string) bool {
	for _, allowedExt := range h.config.AllowedFormats {
		if ext == allowedExt {
			return true
		}
	}
	return false
}

// getTranscriptionOptions extracts transcription options from the request
func (h *Handlers) getTranscriptionOptions(c *gin.Context) *services.TranscriptionOptions {
	options := services.DefaultTranscriptionOptions()

	// Override with form values if provided
	if model := c.PostForm("model"); model != "" {
		options.Model = model
	}
	if language := c.PostForm("language"); language != "" {
		options.Language = language
	}
	if punctuate := c.PostForm("punctuate"); punctuate != "" {
		options.Punctuate = punctuate == "true"
	}
	if diarize := c.PostForm("diarize"); diarize != "" {
		options.Diarize = diarize == "true"
	}
	if smartFormat := c.PostForm("smart_format"); smartFormat != "" {
		options.SmartFormat = smartFormat == "true"
	}
	if includeWords := c.PostForm("include_words"); includeWords != "" {
		options.IncludeWords = includeWords == "true"
	}

	return options
}

// HandleWebSocket handles WebSocket connections for real-time transcription
func (h *Handlers) HandleWebSocket(c *gin.Context) {
	h.websocketService.HandleWebSocket(c.Writer, c.Request)
}

// AnalyzeMedicalText analyzes text for medical terms
func (h *Handlers) AnalyzeMedicalText(c *gin.Context) {
	var request struct {
		Text string `json:"text" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Text is required",
		})
		return
	}

	// Analyze text for medical terms
	medicalTerms := h.medicalVocabService.AnalyzeText(request.Text)
	
	// Normalize vitals
	normalizedText := h.medicalVocabService.NormalizeVitals(request.Text)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"original_text":    request.Text,
			"normalized_text":  normalizedText,
			"medical_terms":    medicalTerms,
			"term_count":       len(medicalTerms),
		},
	})
}

// GetMedicalTermInfo returns information about a specific medical term
func (h *Handlers) GetMedicalTermInfo(c *gin.Context) {
	term := c.Query("term")
	if term == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Term parameter is required",
		})
		return
	}

	termInfo, exists := h.medicalVocabService.GetTermInfo(term)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Medical term not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    termInfo,
	})
}

// SuggestMedicalTermCorrections suggests corrections for medical terms
func (h *Handlers) SuggestMedicalTermCorrections(c *gin.Context) {
	term := c.Query("term")
	if term == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Term parameter is required",
		})
		return
	}

	suggestions := h.medicalVocabService.SuggestCorrections(term)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"term":        term,
			"suggestions": suggestions,
		},
	})
}

// GetWebSocketStats returns WebSocket connection statistics
func (h *Handlers) GetWebSocketStats(c *gin.Context) {
	clientCount := h.websocketService.GetConnectedClients()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"connected_clients": clientCount,
			"service_status":     "active",
		},
	})
}
