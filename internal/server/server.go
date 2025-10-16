package server

import (
	"speech-to-text-backend/internal/handlers"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

// Server represents the HTTP server
type Server struct {
	router *gin.Engine
	logger *logrus.Logger
}

// New creates a new server instance
func New(cfg interface{}, logger *logrus.Logger) *Server {
	// Set Gin mode
	gin.SetMode(gin.ReleaseMode)
	
	router := gin.New()
	
	// Add middleware
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	
	// Add CORS middleware
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		
		c.Next()
	})

	return &Server{
		router: router,
		logger: logger,
	}
}

// SetupRoutes configures all the routes for the server
func (s *Server) SetupRoutes(handlers *handlers.Handlers) {
	// Health check endpoint
	s.router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "speech-to-text-backend"})
	})

	// API routes
	api := s.router.Group("/api/v1")
	{
		api.POST("/transcribe", handlers.TranscribeAudio)
		api.GET("/formats", handlers.GetSupportedFormats)
		
		// WebSocket endpoint
		api.GET("/ws", handlers.HandleWebSocket)
		
		// Medical vocabulary endpoints
		api.POST("/analyze-medical-text", handlers.AnalyzeMedicalText)
		api.GET("/medical-term-info", handlers.GetMedicalTermInfo)
		api.GET("/medical-term-suggestions", handlers.SuggestMedicalTermCorrections)
		
		// WebSocket statistics
		api.GET("/ws-stats", handlers.GetWebSocketStats)
	}
}

// Start starts the HTTP server
func (s *Server) Start(addr string) error {
	return s.router.Run(addr)
}
