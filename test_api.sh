#!/bin/bash

# Test script for Speech-to-Text Backend API
# Make sure the service is running before executing this script

BASE_URL="http://localhost:8080"

echo "🧪 Testing Speech-to-Text Backend API"
echo "======================================"

# Test 1: Health Check
echo "📋 Test 1: Health Check"
curl -s "$BASE_URL/health" | jq '.'
echo ""

# Test 2: Get Supported Formats
echo "📋 Test 2: Get Supported Formats"
curl -s "$BASE_URL/api/v1/formats" | jq '.'
echo ""

# Test 3: Transcribe Audio (if you have a sample audio file)
echo "📋 Test 3: Transcribe Audio"
echo "Note: This test requires a sample audio file named 'sample.wav' in the current directory"
if [ -f "sample.wav" ]; then
    echo "Found sample.wav, testing transcription..."
    curl -s -X POST "$BASE_URL/api/v1/transcribe" \
        -F "audio=@sample.wav" \
        -F "language=en" \
        -F "include_words=true" | jq '.'
else
    echo "⚠️  No sample.wav file found. Skipping transcription test."
    echo "   To test transcription, place an audio file named 'sample.wav' in this directory and run the script again."
fi

echo ""
echo "✅ API tests completed!"
echo ""
echo "💡 Tips:"
echo "   - Make sure your Deepgram API key is set in the .env file"
echo "   - The service should be running on port 8080"
echo "   - For transcription tests, use supported audio formats: .mp3, .wav, .m4a, .flac, .ogg, .webm"
