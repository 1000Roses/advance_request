# Long Polling

## Khái niệm

**Long Polling** là kỹ thuật "hỏi liên tục có gì mới không" — client gửi HTTP request lên server, server **giữ request đó mở** cho đến khi có data mới (hoặc timeout), rồi mới trả response.

```
Client                  Server
  |                       |
  |---GET /events-------->|   Request đang "chờ"
  |                       |   (server hold connection)
  |                       |   ... đợi data ...
  |<--200 OK {data}-------|   Data có rồi → trả về ngay
  |                       |
  |---GET /events-------->|   Client gửi lại ngay
  |   (retry immediately) |   (tiếp tục chờ)
```

## Luồng hoạt động

1. **Client** gửi HTTP request lên server
2. **Server** nhận request, kiểm tra "có data mới không?"
   - Có → trả response ngay
   - Không → `sleep/wait`, giữ connection alive
3. Khi có data hoặc **timeout** (thường 30-60s), server trả response
4. Client nhận response xong → gửi request mới **ngay lập tức**
5. Lặp lại...

## Ưu điểm

- Dùng được ngay với HTTP/1.1, không cần thêm thư viện đặc biệt
- Đơn giản để implement
- Fire-and-forget friendly (dễ retry)

## Nhược điểm

- **Tốn resource**: Mỗi client giữ 1 connection open liên tục
- **Latency**: Giữa 2 poll có khoảng trống (data không đẩy tức thì)
- **Scalability kém**: Server phải handle nhiều concurrent held connections
- **HTTP overhead**: Headers, handshakes... mỗi lần poll

## Phù hợp khi nào

- Hệ thống đơn giản, không cần real-time hoàn toàn
- Browser không hỗ trợ WebSocket
- Cần đẩy data không thường xuyên (VD: notification, email mới)

---

## So sánh nhanh

| | Long Polling | WebSocket | Streaming |
|---|---|---|---|
| **Connection** | Mở/đóng liên tục | Persistent | Persistent |
| **Direction** | Client hỏi, server trả | 2 chiều | 1 chiều (server→client) |
| **Real-time** | Gần real-time | True real-time | True real-time |
| **HTTP only** | ✅ | ❌ (WS protocol) | ✅ |
| **Complexity** | Thấp | Trung bình | Trung bình |
| **Scalability** | Kém | Tốt | Tốt |

---

## Code example (Go)

Xem file `long_polling.go` — implement đơn giản với Gin framework

- Server giữ connection trong 60s hoặc cho đến khi có event
- Client gửi request, đọc response, retry ngay
- Demo đẩy notification từ server

## Chạy thử

```bash
go run long_polling.go
# Server chạy ở :8080
# Endpoints:
#   GET  /poll     - Client long-polling
#   POST /publish  - Server push notification (test)
```

---

## 🚀 Load Test

### Go load_test.go (Khuyên dùng)

```bash
# 1. Chạy server
./long_polling &

# 2. Chạy load test
go run load_test.go -clients=100 -duration=30s

# Kết quả sẽ show:
# - Total requests
# - Success / Timeout / Error counts
# - RPS (requests per second)
# - Avg latency
```

**Flags:**
- `-clients` — Số concurrent clients (mặc định: 50)
- `-duration` — Thời gian test (mặc định: 30s)
- `-broadcast-interval` — Khoảng cách broadcast tự động (mặc định: 500ms)

### Broadcast thủ công

```bash
# Broadcast 1 message
curl -X POST http://localhost:8080/publish -d "message=Hello"

# Broadcast liên tục (0.5s/cái)
while true; do
  curl -s -X POST http://localhost:8080/publish -d "message=Test-$(date +%s)"
  sleep 0.5
done
```

---

## 🧪 Full Test Scenario

```bash
# Terminal 1: Chạy server
go run long_polling.go

# Terminal 2: Chạy load test
go run load_test.go -clients=100 -duration=60s

# Terminal 3: (Optional) Broadcast thêm data
while true; do
  curl -s -X POST http://localhost:8080/publish -d "message=LoadTest-$(date +%s)" &
  sleep 0.2
done
```

---

## 📊 Metrics cần theo dõi

- **RPS** — Requests per second (throughput)
- **Latency** — Response time trung bình
- **Error rate** — % request thất bại
- **Active clients** — Số clients đang polling

```bash
# Xem đang có bao nhiêu clients đang chờ
curl http://localhost:8080/clients
```