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

- **Simple**: Chỉ cần HTTP, không cần thư viện đặc biệt
- **Auto-reconnect**: Browser tự động reconnect khi mất kết nối
- **HTTP/2 compatible**: Tận dụng multiplexed streams
- **Fire-and-forget**: Server gửi, không cần ACK từ client

## Nhược điểm

- **Chỉ 1 chiều**: Server → Client (muốn gửi lên phải dùng fetch/XHR riêng)
- **Connection limit**: HTTP/1.1 giới hạn ~6 connections per domain
- **Text-only**: Binary data phải encode sang base64

## Phù hợp khi nào

- **Dashboard**: Hiển thị real-time metrics, logs
- **Notifications**: Thông báo từ server
- **Live feeds**: Tin tức, social media feeds
- **Progress updates**: Upload/download progress

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

---

## 🚀 Load Test / Bắn dữ liệu

### Cách 1: Apache Bench (ab)

```bash
# Cài đặt: apt install apache2-utils

# Test với nhiều concurrent connections
ab -n 1000 -c 50 http://localhost:8080/events

# Note: ab sẽ timeout hoặc receive data liên tục
```

### Cách 2: WRK với Lua script

```bash
# Cài đặt: apt install wrk

# Tạo script để count events nhận được
wrk -t10 -c100 -d30s http://localhost:8080/events
```

### Cách 3: Go load test script

```bash
# Chạy file load_test.go
go run load_test.go -clients=100 -duration=30s
```

File `load_test.go` đã có sẵn — bắn nhiều concurrent SSE clients đồng thời.

### Cách 4: Auto-broadcast

```bash
# Broadcast liên tục để simulate real data
while true; do
  curl -s -X POST "http://localhost:8080/broadcast?msg=LoadTest-$(date +%s)" &
  sleep 0.5
done

# Burst test
for i in {1..100}; do
  curl -s -X POST "http://localhost:8080/broadcast?msg=Burst-$i" &
done
wait
```

### Cách 5: Vegeta

```bash
go install github.com/tsenart/vegeta@latest

# SSE là persistent connection nên dùng vegeta để test throughput
echo "GET http://localhost:8080/events" | vegeta attack -rate=50 -duration=30s | vegeta report
```

### Cách 6: Curl nhiều clients

```bash
# Mở nhiều terminal hoặc background
for i in {1..50}; do
  curl -N http://localhost:8080/events &
done
sleep 30
killall curl
```

### Cách 7: Hey (HTTP load test)

```bash
npm install -g hey

# Test persistent connections
hey -n 1000 -c 50 -z 30s http://localhost:8080/events
```

---

## 📊 Metrics cần theo dõi

- **Active connections**: Số clients đang listen
- **Events/second**: Bao nhiêu events được gửi
- **Broadcast success rate**: Bao nhiêu % clients nhận được broadcast
- **Connection duration**: Connections sống được bao lâu

```bash
# Theo dõi active SSE clients real-time
watch -n1 'curl -s http://localhost:8080/clients | jq .'
```

---

## 🧪 Full Load Test Scenario

```bash
# Terminal 1: Chạy server
go run streaming.go

# Terminal 2: Bắn broadcast liên tục (100 msgs/s)
for i in {1..1000}; do
  curl -s -X POST "http://localhost:8080/broadcast?msg=LoadTest-$i" &
  [ $((i % 10)) -eq 0 ] && sleep 0.1
done

# Terminal 3: Chạy load test với 200 concurrent SSE clients
go run load_test.go -clients=200 -duration=60s

# Terminal 4: Theo dõi clients
watch -n1 'curl -s http://localhost:8080/clients'
```

---

## 🧪 Stress Test - Đẩy đến giới hạn

```bash
# Đẩy 500 concurrent SSE connections
go run load_test.go -clients=500 -duration=120s

# Kết hợp broadcast burst
while true; do
  for i in {1..50}; do
    curl -s -X POST "http://localhost:8080/broadcast?msg=Burst-$i" &
  done
  wait
  sleep 0.5
done
```