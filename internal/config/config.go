package config

import (
	"os"
)

// Config holds all configuration for our application
type Config struct {
	DeepgramAPIKey string
	ServerPort     string
	MaxFileSize    int64
	AllowedFormats []string
}

// Load reads configuration from environment variables
func Load() *Config {
	return &Config{
		DeepgramAPIKey: getEnv("DEEPGRAM_API_KEY", ""),
		ServerPort:     getEnv("PORT", "8080"),
		MaxFileSize:    50 * 1024 * 1024, // 50MB
		AllowedFormats: []string{".mp3", ".wav", ".m4a", ".flac", ".ogg", ".webm"},
	}
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
