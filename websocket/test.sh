#!/bin/bash
# WebSocket Test Script

echo "=== WebSocket Test ==="
echo ""

# Start server in background
cd "$(dirname "$0")"
./websocket &
SERVER_PID=$!
sleep 2

echo "Server started (PID: $SERVER_PID)"
echo ""

# Test HTTP broadcast
echo "📡 Test: HTTP broadcast to WebSocket clients"
curl -s "http://localhost:8080/broadcast?msg=Hello from HTTP!"

echo ""
echo "📋 Current WebSocket clients:"
curl -s "http://localhost:8080/clients" | jq .

echo ""
echo "To test WebSocket manually:"
echo "  1. Open browser console"
echo "  2. ws = new WebSocket('ws://localhost:8080/ws')"
echo "  3. ws.onmessage = (e) => console.log(e.data)"
echo "  4. ws.send('Hello!')"
echo ""
echo "Press Ctrl+C to stop server..."

# Wait for Ctrl+C
wait $SERVER_PID