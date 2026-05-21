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

## 🚀 Load Test

### Go load_test.go (Khuyên dùng)

```bash
# 1. Chạy server
./websocket &

# 2. Chạy load test
go run load_test.go -clients=100 -duration=30s

# Kết quả sẽ show:
# - Connected clients
# - Messages sent/received
# - Throughput (msg/s)
```

**Flags:**
- `-clients` — Số concurrent WS connections (mặc định: 50)
- `-duration` — Thời gian test (mặc định: 30s)
- `-broadcast-interval` — Khoảng cách broadcast tự động (mặc định: 500ms)

### HTTP Broadcast

```bash
# Broadcast message đến tất cả WS clients
curl "http://localhost:8080/broadcast?msg=Hello"
```

---

## 🧪 Full Test Scenario

```bash
# Terminal 1: Chạy server
go run websocket.go

# Terminal 2: Chạy load test
go run load_test.go -clients=100 -duration=60s

# Terminal 3: (Optional) Broadcast thêm từ HTTP
while true; do
  curl -s "http://localhost:8080/broadcast?msg=LoadTest-$(date +%s)" &
  sleep 0.2
done
```

---

## 📊 Metrics cần theo dõi

- **Active connections** — Số clients đang kết nối
- **Messages/second** — Throughput
- **Latency** — Round-trip time
- **Error count** — Số lỗi connection/message

```bash
# Xem đang có bao nhiêu clients kết nối
curl http://localhost:8080/clients
```