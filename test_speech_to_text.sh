#!/bin/bash

# Test script for Speech-to-Text Backend
echo "Testing Speech-to-Text Backend..."

# Test health endpoint
echo "1. Testing health endpoint..."
curl -s https://api-backend.chatess.com/health | jq .

# Test supported formats
echo -e "\n2. Testing supported formats..."
curl -s https://api-backend.chatess.com/api/v1/formats | jq .

# Test medical text analysis
echo -e "\n3. Testing medical text analysis..."
curl -s -X POST https://api-backend.chatess.com/api/v1/analyze-medical-text \
  -H "Content-Type: application/json" \
  -d '{"text": "Patient has blood pressure of one twenty over eighty and heart rate of seventy two beats per minute"}' | jq .

# Test medical term info
echo -e "\n4. Testing medical term info..."
curl -s "https://api-backend.chatess.com/api/v1/medical-term-info?term=blood%20pressure" | jq .

# Test medical term suggestions
echo -e "\n5. Testing medical term suggestions..."
curl -s "https://api-backend.chatess.com/api/v1/medical-term-suggestions?term=blood%20presure" | jq .

# Test WebSocket stats
echo -e "\n6. Testing WebSocket stats..."
curl -s https://api-backend.chatess.com/api/v1/ws-stats | jq .

echo -e "\nAll tests completed!"
