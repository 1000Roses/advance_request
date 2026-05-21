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