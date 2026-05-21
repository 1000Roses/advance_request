package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ============================================================
// LONG POLLING - Chi tiết từng phần
// ============================================================
// Long Polling là kỹ thuật:
//   Client gửi request lên server, server GIỮ request đó lại
//   (không trả response ngay) cho đến khi:
//   1. Có data mới → trả response ngay
//   2. Timeout (VD: 60s) → trả response rỗng
//   Client nhận xong → gửi request mới ngay
//
// Điểm mấu chốt: Server dùng channel để "thức tỉnh" goroutine
// đang giữ request khi có event đến
// ============================================================

// --- KHU VỰC 1: Kiểu dữ liệu và Global State ---

// EventChannel: mỗi client khi poll sẽ có 1 channel riêng
// Khi server có notification mới, nó sẽ gửi vào TẤT CẢ channel
// của các client đang chờ
type EventChannel struct {
	// Chứa notification để gửi cho client
	// Buffer 1 vì chỉ cần gửi 1 event là đủ
	// Client sẽ quay lại poll ngay sau khi nhận
	Event chan string
}

// Manager quản lý tất cả client đang kết nối
// Dùng mutex vì có thể nhiều goroutine truy cập đồng thời
var (
	clientManager = struct {
		mu      sync.RWMutex
		clients map[string]*EventChannel // map clientID -> channel
	}{
		clients: make(map[string]*EventChannel),
	}
)

// getClientChannel: Lấy hoặc tạo channel mới cho 1 client
// - Nếu client đã có channel → trả về channel cũ
// - Nếu chưa có → tạo channel mới, lưu vào map
func getClientChannel(clientID string) *EventChannel {
	clientManager.mu.Lock()
	defer clientManager.mu.Unlock()

	// Kiểm tra đã có channel chưa
	if ch, exists := clientManager.clients[clientID]; exists {
		return ch
	}

	// Chưa có → tạo mới
	ch := &EventChannel{
		Event: make(chan string, 1), // buffered channel,容量 1
	}
	clientManager.clients[clientID] = ch
	return ch
}

// removeClientChannel: Xóa channel khi client ngắt kết nối
// Quan trọng để tránh memory leak - goroutine đang chờ trên channel
// sẽ bị bỏ rơi nếu không đóng channel
func removeClientChannel(clientID string) {
	clientManager.mu.Lock()
	defer clientManager.mu.Unlock()

	if ch, exists := clientManager.clients[clientID]; exists {
		close(ch.Event)    // Đóng channel để thoát khỏi receive operation
		delete(clientManager.clients, clientID)
	}
}

// --- KHU VỰC 2: Broadcast Function ---

// broadcastEvent: Gửi event đến TẤT CẢ clients đang chờ
// Dùng select với default case để tránh blocking nếu channel full
func broadcastEvent(message string) {
	clientManager.mu.RLock()
	defer clientManager.mu.RUnlock()

	// Lặp qua tất cả clients và gửi event
	// Dùng non-blocking send (select + default) để không block
	// nếu 1 client đã nhận rồi (buffer chỉ có 1)
	for _, ch := range clientManager.clients {
		select {
		case ch.Event <- message:
			// Gửi thành công
		default:
			// Channel đã có message, bỏ qua client này
			// Client sẽ nhận message tiếp theo trong poll tiếp
		}
	}
}

// --- KHU VỰC 3: HTTP Handlers ---

// Handler: POST /publish
// Dùng để test - gửi notification đến tất cả clients
// curl -X POST http://localhost:8080/publish -d "message=Hello"
func publishHandler(c *gin.Context) {
	// Lấy message từ body
	message := c.PostForm("message")
	if message == "" {
		message = "Default notification"
	}

	// Log để debug
	log.Printf("[PUBLISH] Broadcasting: %s to %d clients",
		message, len(clientManager.clients))

	// Broadcast đến tất cả clients
	broadcastEvent(message)

	c.JSON(http.StatusOK, gin.H{
		"status":  "published",
		"message": message,
		"clients": len(clientManager.clients),
	})
}

// Handler: GET /poll
// Đây là handler CHÍNH cho long polling
//
// Luồng xử lý:
// 1. Lấy client ID (từ query param hoặc tạo mới)
// 2. Đăng ký client vào manager (lấy/tạo channel)
// 3. Dùng select với timeout 60s
//    - Nếu có event → trả về ngay
//    - Nếu timeout → trả response rỗng
//    - Nếu client disconnect → thoát và dọn dẹp
func pollHandler(c *gin.Context) {
	// Bước 1: Xác định client ID
	// Có thể truyền ?client_id=xxx hoặc dùng IP làm ID
	clientID := c.Query("client_id")
	if clientID == "" {
		// Fallback: dùng IP + User-Agent làm identifier
		clientID = fmt.Sprintf("%s-%s", c.ClientIP(), c.Request.UserAgent())
	}

	// Bước 2: Đăng ký client, lấy channel
	ch := getClientChannel(clientID)

	// Bước 3: Log khi client bắt đầu poll
	log.Printf("[POLL] Client connected: %s", clientID)

	// Bước 4: Cleanup khi function kết thúc
	// defer đảm bảo channel được xóa khỏi manager
	// nhưng KHÔNG đóng channel ở đây vì có thể còn event đang gửi
	defer func() {
		log.Printf("[POLL] Client disconnected: %s", clientID)
	}()

	// Bước 5: Dùng select để đợi event HOẶC timeout
	// Đây là "secret sauce" của long polling
	select {
	case event, ok := <-ch.Event:
		// Có event gửi đến
		if !ok {
			// Channel đã đóng (client bị xóa)
			c.JSON(http.StatusOK, gin.H{
				"event":    "",
				"received": false,
				"reason":   "channel_closed",
			})
			return
		}
		// Trả về event cho client
		log.Printf("[POLL] Sending event to %s: %s", clientID, event)
		c.JSON(http.StatusOK, gin.H{
			"event":    event,
			"received": true,
			"client":   clientID,
		})

	case <-time.After(60 * time.Second):
		// TIMEOUT sau 60s
		// Server trả response "không có gì mới" để client quay lại poll
		log.Printf("[POLL] Timeout for client: %s", clientID)
		c.JSON(http.StatusOK, gin.H{
			"event":    "",
			"received": false,
			"reason":   "timeout",
		})
	}
	// Khi response được gửi, client sẽ:
	// 1. Nhận được JSON response
	// 2. Gửi request /poll MỚI ngay lập tức
	// → Lặp lại vòng lặp
}

// Handler: GET /clients - Debug endpoint để xem có bao nhiêu client đang chờ
func listClientsHandler(c *gin.Context) {
	clientManager.mu.RLock()
	defer clientManager.mu.RUnlock()

	clients := make([]string, 0, len(clientManager.clients))
	for id := range clientManager.clients {
		clients = append(clients, id)
	}

	c.JSON(http.StatusOK, gin.H{
		"count":   len(clients),
		"clients": clients,
	})
}

// --- KHU VỰC 4: Main ---

func main() {
	// Tạo Gin router
	gin.SetMode(gin.ReleaseMode) // Mode release để ít log
	r := gin.Default()

	// Các endpoints
	r.GET("/poll", pollHandler)         // Client gọi endpoint này để "hỏi có gì mới không"
	r.POST("/publish", publishHandler)  // Server gọi endpoint này để push notification
	r.GET("/clients", listClientsHandler)

	fmt.Println("Long Polling Server running on :8080")
	fmt.Println("Try these commands in another terminal:")
	fmt.Println("  curl http://localhost:8080/poll?client_id=user1")
	fmt.Println("  curl -X POST http://localhost:8080/publish -d 'message=Hello!'")

	log.Fatal(r.Run(":8080"))
}