#!/bin/bash
# SSE Streaming Test Script

echo "=== SSE Streaming Test ==="
echo ""

# Start server in background
cd "$(dirname "$0")"
./streaming &
SERVER_PID=$!
sleep 2

echo "Server started (PID: $SERVER_PID)"
echo ""

# Test 1: SSE stream (background, will receive events)
echo "📡 Test 1: Start SSE stream (Ctrl+C to stop)"
echo "Open another terminal and run:"
echo "  curl -N http://localhost:8080/events"
echo ""
echo "Or use browser console:"
echo "  const source = new EventSource('http://localhost:8080/events')"
echo "  source.onmessage = (e) => console.log('MSG:', e.data)"
echo ""

# Test 2: HTTP broadcast
echo "📡 Test 2: HTTP broadcast"
curl -s -X POST "http://localhost:8080/broadcast?msg=Test-broadcast"

echo ""
echo "📋 Current SSE clients:"
curl -s "http://localhost:8080/clients" | jq .

echo ""
echo "📡 Demo stream (with auto counter):"
echo "  curl -N http://localhost:8080/stream/demo"
echo ""
echo "Press Ctrl+C to stop server..."

wait $SERVER_PID