# Server-Sent Events (Streaming)

## Khái niệm

**Server-Sent Events (SSE)** là cách để server **PUSH data liên tục** đến client qua HTTP connection đã được upgrade.

```
HTTP/1.1 200 OK
Content-Type: text/event-stream
Cache-Control: no-cache
Connection: keep-alive

event: message
data: {"text": "Hello!"}

event: message
data: {"text": "World!"}

event: ping
data: heartbeat
```

## Luồng hoạt động

1. **Client** mở HTTP connection đến endpoint `/events`
2. **Server** giữ connection **KHÔNG ĐÓNG**
3. Khi có data mới, server gửi dạng `event: ...\ndata: ...\n\n`
4. Client nhận qua `EventSource` API (browser built-in)
5. Connection vẫn mở → server tiếp tục gửi khi cần
6. **Timeout/reconnect**: Nếu connection chết, browser tự động reconnect

```
Client                  Server
  |                       |
  |---GET /events-------->|   Connection OPEN
  |                       |   ... đợi ...
  |<--event: msg----------|   data 1
  |   data: Hello!        |
  |                       |   ... đợi ...
  |<--event: msg----------|   data 2
  |   data: World!        |
  |                       |
  |    (keep-alive)       |
  |<--event: ping--------|   heartbeat (15-30s)
  |   data: heartbeat    |
  |                       |   ... đợi ...
```

## SSE vs WebSocket

| | SSE | WebSocket |
|---|---|---|
| **Direction** | Server → Client only | 2 chiều |
| **Protocol** | HTTP | `ws://` (TCP-based) |
| **Browser Support** | Native EventSource | Native WebSocket API |
| **Auto-reconnect** | ✅ Built-in | ❌ Phải tự implement |
| **Max connections** | ~6 per domain (HTTP/1.1) | Multiple |
| **Binary data** | Base64 encoded | Native binary frames |
| **Firewall/proxy** | Không vấn đề | Có thể bị chặn |

## Ưu điểm

- **Simple**: Chỉ cần HTTP, không cần protocol đặc biệt
- **Auto-reconnect**: Browser tự động reconnect khi mất kết nối
- **Auto-reconnect với retry**: Có thể gửi custom retry interval
- **HTTP/2 compatible**: Tận dụng multiplexed streams
- **Fire-and-forget**: Server gửi, không cần ACK từ client

## Nhược điểm

- **Chỉ 1 chiều**: Server → Client (muốn gửi lên phải dùng fetch/XHR riêng)
- **Connection limit**: HTTP/1.1 giới hạn ~6 connections per domain
- **Text-only**: Binary data phải encode sang base64
- **No IE/Edge legacy**: EventSource không có trong old IE

## Phù hợp khi nào

- **Dashboard**: Hiển thị real-time metrics, logs
- **Notifications**: Thông báo từ server
- **Live feeds**: Tin tức, social media feeds
- **Progress updates**: Upload/download progress
- **Khi không cần 2-way communication**

---

## Code example (Go)

Xem file `streaming.go` — implement SSE với Gin framework

- Server gửi messages định kỳ (countdown, random numbers)
- Broadcast event đến tất cả clients
- Heartbeat để giữ connection alive
- Client auto-reconnect

## Chạy thử

```bash
go run streaming.go
# Server chạy ở :8080
# SSE endpoint: http://localhost:8080/events

# Test bằng browser:
# const source = new EventSource("http://localhost:8080/events")
# source.onmessage = (e) => console.log(e.data)
# source.addEventListener("notification", (e) => console.log(e.data))
```