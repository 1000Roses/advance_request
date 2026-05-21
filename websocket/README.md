# WebSocket

## Khái niệm

**WebSocket** là giao thức **persistent, bidirectional** communication giữa client và server qua **một TCP connection duy nhất**.

```
HTTP Handshake (Upgrade Request)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Client                          Server
  |                               |
  |--GET /ws (with Upgrade)------>|
  |<--101 Switching Protocols -----|
  |                               |
  │     WebSocket Connection      │
  │◄══════════════════════════►│
  │   Full-duplex, 2-way traffic  │
  |                               |
```

## Điểm khác biệt với HTTP

| | HTTP | WebSocket |
|---|---|---|
| **Connection** | Request-Response | Persistent |
| **Direction** | Client hỏi, server trả | 2 chiều bất kỳ lúc nào |
| **Overhead** | Headers mỗi request | Handshake 1 lần, sau đó frame nhẹ |
| **Server Push** | ❌ (cần polling/SSE) | ✅ Có |
| **Fire-and-Forget** | Khó | ✅ Dễ |

## WebSocket vs Long Polling

```
LONG POLLING:
Client: ───GET /poll───> [chờ 30s] <───200 OK────
Client: ───GET /poll───> [chờ 30s] <───200 OK────
Client: ───GET /poll───> [chờ 30s] <───200 OK────
        (request mới ngay sau response)

WEBSOCKET:
Client: ════════════════════════════════════════
              1 connection, push lúc nào cũng được
```

## Ưu điểm

- **True real-time**: Server đẩy data ngay lập tức
- **Low latency**: Không có request/response overhead
- **Persistent**: Giữ nguyên connection, không reconnect liên tục
- **Bidirectional**: Cả client và server đều gửi được

## Nhược điểm

- **Complexity cao hơn**: Cần thư viện, cần handle reconnection logic
- **Stateless khó hơn**: Connection stateful, khó scale horizontal
- **Proxy/Firewall**: Một số proxy có thể block WS
- **Browser support**: Hơi khác nhau giữa các browser

## Phù hợp khi nào

- Ứng dụng cần **real-time 2 chiều** (chat, game, collaborative editing)
- Data thay đổi **liên tục** (stock prices, live sports)
- Cần **low latency** (multiplayer games, trading)

---

## Code example (Go)

Xem file `websocket.go` — implement với `gorilla/websocket`

- Echo server (nhận message, gửi lại cho client)
- Broadcast đến nhiều clients
- Heartbeat/ping-pong để detect disconnect
- Connection cleanup khi client ngắt

## Chạy thử

```bash
go run websocket.go
# Server chạy ở :8080
# WebSocket endpoint: ws://localhost:8080/ws

# Test bằng browser console:
# ws = new WebSocket("ws://localhost:8080/ws")
# ws.onmessage = (e) => console.log(e.data)
# ws.send("Hello!")
```

---

## 🚀 Load Test / Bắn dữ liệu

### Cách 1: WebSocket benchmarks

```bash
# Test với nhiều concurrent connections
# Dùng tool: https://github.com/vi/websocat
apt install websocat

# Connect nhiều clients
websocat ws://localhost:8080/ws &
websocat ws://localhost:8080/ws &
websocat ws://localhost:8080/ws &
```

### Cách 2: Tsung (Erlang-based distributed load test)

```bash
# Cài đặt: apt install tsung
# Config file: wsload.xml
# Chạy: tsung -f wsload.xml start
```

### Cách 3: Artillery (Node.js load test)

```bash
npm install -g artillery

# Config ws-load.yml
artillery quick --count 50 --num 100 ws://localhost:8080/ws
```

### Cách 4: Ghz (Go gRPC/WebSocket benchmark)

```bash
# Cài đặt: go install github.com/boz/ghoz@latest
ghz wss://localhost:8080/ws -n 1000 -c 50
```

### Cách 5: Go load test script (custom)

```bash
# Chạy file load_test.go để bắn nhiều concurrent WS clients
go run load_test.go -clients=100 -duration=30s -server=localhost:8080
```

File `load_test.go` đã có sẵn.

### Cách 6: Auto-broadcast liên tục

```bash
# HTTP broadcast sẽ gửi đến tất cả WS clients
while true; do
  curl -s "http://localhost:8080/broadcast?msg=LoadTest-$(date +%s)"
  sleep 0.5
done

# Burst test
for i in {1..100}; do
  curl -s "http://localhost:8080/broadcast?msg=Burst-$i" &
done
```

### Cách 7: wscat (WebSocket cat)

```bash
npm install -g wscat

# Multiple connections
wscat -c ws://localhost:8080/ws &
wscat -c ws://localhost:8080/ws &
wscat -c ws://localhost:8080/ws &
```

---

## 📊 Metrics cần theo dõi

- **Active connections**: Số clients đang kết nối
- **Messages/second**: Throughput
- **Latency**: Round-trip time
- **Connection duration**: Bao lâu clients stay connected
- **Broadcast success rate**: Bao nhiêu % clients nhận được broadcast

```bash
# Theo dõi active clients real-time
watch -n1 'curl -s http://localhost:8080/clients | jq .'
```

---

## 🧪 Full Load Test Scenario

```bash
# Terminal 1: Chạy server
go run websocket.go

# Terminal 2: Bắn HTTP broadcast liên tục
while true; do
  curl -s "http://localhost:8080/broadcast?msg=LoadTest-$(date +%s)" &
  sleep 0.1
done

# Terminal 3: Chạy load test với 100 concurrent clients
go run load_test.go -clients=100 -duration=60s

# Terminal 4: Theo dõi clients
watch -n1 'curl -s http://localhost:8080/clients'
```