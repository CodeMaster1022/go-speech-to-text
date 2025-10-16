package main

import (
	"log"
	"os"

	"speech-to-text-backend/internal/config"
	"speech-to-text-backend/internal/handlers"
	"speech-to-text-backend/internal/server"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
)

func main() {
	// Load environment variables from .env file
	if err := godotenv.Load(); err != nil {
		logrus.Warn("No .env file found, using system environment variables")
	}

	// Load configuration
	cfg := config.Load()

	// Create logger
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	// Create server
	srv := server.New(cfg, logger)

	// Create handlers
	handlers := handlers.New(cfg, logger)

	// Setup routes
	srv.SetupRoutes(handlers)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	logger.Infof("Starting server on port %s", port)
	if err := srv.Start(":" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
