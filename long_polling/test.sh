#!/bin/bash
# Long Polling Test Script

echo "=== Long Polling Test ==="
echo ""

# Start server in background
cd "$(dirname "$0")"
./long_polling &
SERVER_PID=$!
sleep 2

echo "Server started (PID: $SERVER_PID)"
echo ""

# Test 1: Poll (should timeout after 60s, but we broadcast first)
echo "📡 Test 1: Start polling (client_id=user1)"
curl -s "http://localhost:8080/poll?client_id=user1" &
POLL_PID=$!

sleep 2

# Test 2: Broadcast a message
echo ""
echo "📡 Test 2: Broadcast message"
curl -s -X POST "http://localhost:8080/publish" -d "message=Hello from broadcast!"

# Wait for poll to receive
sleep 2

# Kill poll
kill $POLL_PID 2>/dev/null

echo ""
echo "=== Test 3: Multiple clients ==="
curl -s "http://localhost:8080/poll?client_id=user2" &
curl -s "http://localhost:8080/poll?client_id=user3" &

sleep 1
echo ""
echo "📡 Broadcasting to all clients..."
curl -s -X POST "http://localhost:8080/publish" -d "message=Multi-client test!"

sleep 2

# List clients
echo ""
echo "📋 Current clients:"
curl -s "http://localhost:8080/clients" | jq .

# Cleanup
kill $SERVER_PID 2>/dev/null
echo ""
echo "Done!"