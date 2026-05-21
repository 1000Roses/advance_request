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

## 🚀 Load Test / Bắn dữ liệu

### Cách 1: Apache Bench (ab)

```bash
# Cài đặt nếu chưa có: apt install apache2-utils

# Test 100 request, 10 concurrent
ab -n 100 -c 10 http://localhost:8080/poll

# Test 1000 request, 50 concurrent (để xem performance)
ab -n 1000 -c 50 http://localhost:8080/poll
```

### Cách 2: WRK (Khuyên dùng)

```bash
# Cài đặt: apt install wrk

# Test với 100 threads, 1000 connections trong 30s
wrk -t12 -c100 -d30s http://localhost:8080/poll

# Test với custom script (xem wrk/scripts/poll.lua)
wrk -t4 -c200 -d30s -s poll.lua http://localhost:8080/poll
```

### Cách 3: Siege

```bash
# Cài đặt: apt install siege

# Simulate 50 users, 30 seconds
siege -c50 -t30s http://localhost:8080/poll
```

### Cách 4: Go bắn nhiều clients

```bash
# Chạy file load_test.go để bắn nhiều concurrent clients
go run load_test.go -clients=100 -duration=30s
```

File `load_test.go` đã có sẵn — bắn nhiều long-polling clients đồng thời.

### Cách 5: Auto-broadcast

```bash
# Broadcast liên tục mỗi 0.5s để simulate real data
while true; do
  curl -s -X POST http://localhost:8080/publish -d "message=Test-$(date +%s)"
  sleep 0.5
done

# Hoặc nhiều messages cùng lúc
for i in {1..100}; do
  curl -s -X POST http://localhost:8080/publish -d "message=Burst-$i" &
done
wait
```

### Cách 6: Vegeta (Go load test tool)

```bash
# Cài đặt
go install github.com/tsenart/vegeta@latest

# Attack 50 requests/second trong 30s
echo "GET http://localhost:8080/poll" | vegeta attack -rate=50 -duration=30s | vegeta report

# Encode kết quả
echo "GET http://localhost:8080/poll" | vegeta attack -rate=100 -duration=30s | vegeta encode | jq .
```

---

## 📊 Metrics cần theo dõi

- **Latency**: Response time trung bình, p95, p99
- **Throughput**: Requests/second
- **Error rate**: Số request thất bại
- **Active connections**: Số clients đang polling
- **Timeout rate**: Bao nhiêu request bị timeout

```bash
# Theo dõi real-time
watch -n1 'curl -s http://localhost:8080/clients | jq .'
```

---

## 🧪 Full Load Test Scenario

```bash
# Terminal 1: Chạy server
go run long_polling.go

# Terminal 2: Bắn broadcast liên tục
while true; do
  curl -s -X POST http://localhost:8080/publish -d "message=LoadTest-$(date +%s)"
  sleep 0.1
done &

# Terminal 3: Chạy load test với wrk
wrk -t10 -c200 -d60s http://localhost:8080/poll

# Terminal 4: Theo dõi clients
watch -n1 'curl -s http://localhost:8080/clients'
```