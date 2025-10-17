# Speech-to-Text Backend

A high-performance Go-based backend service for converting audio files to text using Deepgram's advanced speech recognition API. This service is containerized with Docker for easy deployment and scaling.

## Features

- 🎵 **Multiple Audio Formats**: Supports MP3, WAV, M4A, FLAC, OGG, and WebM
- 🚀 **High Performance**: Built with Go for fast processing
- 🐳 **Docker Ready**: Fully containerized with Docker Compose
- 📊 **Detailed Results**: Returns confidence scores and word-level timestamps
- 🔧 **Configurable**: Customizable transcription options
- 🛡️ **Production Ready**: Includes health checks, logging, and error handling
- 🌐 **CORS Enabled**: Ready for frontend integration

## Prerequisites

- Go 1.21 or higher
- Docker and Docker Compose
- Deepgram API key ([Get one here](https://console.deepgram.com/))

## Quick Start

### 1. Clone and Setup

```bash
git clone <your-repo-url>
cd backend
cp env.example .env
```

### 2. Configure Environment

Edit `.env` file and add your Deepgram API key:

```bash
DEEPGRAM_API_KEY=your_actual_deepgram_api_key_here
PORT=8080
```

### 3. Run with Docker (Recommended)

```bash
# Build and start the service
docker-compose up --build

# Or run in background
docker-compose up -d --build
```

### 4. Run Locally (Development)

```bash
# Install dependencies
go mod download

# Run the service
go run main.go
```

The service will be available at `https://api-backend.chatess.com`

## API Endpoints

### Health Check
```http
GET /health
```

### Get Supported Formats
```http
GET /api/v1/formats
```

**Response:**
```json
{
  "success": true,
  "data": {
    "supported_formats": [".mp3", ".wav", ".m4a", ".flac", ".ogg", ".webm"],
    "max_file_size_mb": 50
  }
}
```

### Transcribe Audio
```http
POST /api/v1/transcribe
```

**Form Data:**
- `audio` (required): Audio file
- `model` (optional): Deepgram model (default: "nova-2")
- `language` (optional): Language code (default: "en")
- `punctuate` (optional): Add punctuation (default: true)
- `diarize` (optional): Speaker diarization (default: false)
- `smart_format` (optional): Smart formatting (default: true)
- `include_words` (optional): Include word-level data (default: false)

**Example using curl:**
```bash
curl -X POST https://api-backend.chatess.com/api/v1/transcribe \
  -F "audio=@sample.wav" \
  -F "language=en" \
  -F "include_words=true"
```

**Response:**
```json
{
  "success": true,
  "data": {
    "filename": "sample.wav",
    "text": "Hello, this is a test transcription.",
    "confidence": 0.95,
    "words": [
      {
        "word": "Hello",
        "confidence": 0.98,
        "start": 0.5,
        "end": 0.8
      }
    ],
    "word_count": 7
  }
}
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DEEPGRAM_API_KEY` | Your Deepgram API key | Required |
| `PORT` | Server port | 8080 |
| `MAX_FILE_SIZE_MB` | Maximum file size in MB | 50 |

### Deepgram Models

The service supports various Deepgram models:
- `nova-2` (default): Latest general-purpose model
- `nova`: Previous generation model
- `enhanced`: Enhanced accuracy model
- `base`: Base model for faster processing

## Docker Commands

```bash
# Build the image
docker build -t speech-to-text-backend .

# Run the container
docker run -p 8080:8080 -e DEEPGRAM_API_KEY=your_key speech-to-text-backend

# View logs
docker-compose logs -f

# Stop the service
docker-compose down

# Rebuild and restart
docker-compose up --build --force-recreate
```

## Development

### Project Structure

```
backend/
├── main.go                 # Application entry point
├── go.mod                  # Go module definition
├── Dockerfile              # Docker configuration
├── docker-compose.yml      # Docker Compose setup
├── internal/
│   ├── config/            # Configuration management
│   ├── handlers/          # HTTP request handlers
│   ├── services/          # Business logic (Deepgram client)
│   └── server/            # HTTP server setup
└── README.md              # This file
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific test
go test ./internal/services
```

### Code Style

This project follows Go best practices:
- Standard Go project layout
- Proper error handling
- Structured logging
- Clean separation of concerns

## Production Deployment

### Environment Setup

1. Set production environment variables
2. Use a reverse proxy (nginx) for SSL termination
3. Configure proper logging and monitoring
4. Set up health checks and alerts

### Scaling

The service is stateless and can be horizontally scaled:
- Use Docker Swarm or Kubernetes
- Configure load balancing
- Monitor Deepgram API rate limits

## Troubleshooting

### Common Issues

1. **"Deepgram API key is required"**
   - Ensure `DEEPGRAM_API_KEY` is set in your environment

2. **"Unsupported audio format"**
   - Check that your audio file is in a supported format
   - Use the `/api/v1/formats` endpoint to see supported formats

3. **"Transcription failed"**
   - Verify your Deepgram API key is valid
   - Check audio file quality and format
   - Review logs for detailed error messages

### Logs

View application logs:
```bash
# Docker Compose
docker-compose logs -f

# Local development
go run main.go
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

This project is licensed under the MIT License.

## Support

For issues and questions:
- Check the [Deepgram Documentation](https://developers.deepgram.com/)
- Review the logs for error details
- Open an issue in this repository
