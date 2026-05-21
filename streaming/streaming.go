package main

import (
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// ============================================================
// SERVER-SENT EVENTS (STREAMING) - Chi tiết từng phần
// ============================================================
// SSE (Server-Sent Events) là cách để server PUSH data liên tục
// qua HTTP connection mà KHÔNG cần client hỏi lại (polling).
//
// Điểm khác Long Polling:
//   Long Polling: Request → đợi → Response → Request mới → đợi...
//   SSE:           Request → đợi → Response → đợi → Response → đợi...
//                  (connection PERSISTENT, không đóng giữa 2 responses)
//
// Format của 1 event:
//   event: <event_type>\n
//   data: <payload>\n\n
//
// 2 newlines (\n\n) là delimiter cho biết event hoàn chỉnh
//
// Điểm khác WebSocket:
//   SSE: Chỉ server → client (1 chiều)
//   WS:  Cả 2 chiều (bidirectional)
//   SSE: Dùng HTTP thuần, WS: Cần protocol riêng (ws://)
//   SSE: Browser tự reconnect, WS: Phải tự handle
// ============================================================

// --- KHU VỰC 1: Client Manager ---

// Client: đại diện cho 1 SSE connection (1 client đang listen)
type SSEClient struct {
	// ID của client
	ID string

	// Channel để gửi events đến client
	// Buffered channel với容量 10 để tránh block nếu client đọc chậm
	Events chan string

	// Done channel để signal client disconnect
	Done chan struct{}
}

// SSEClients: Global manager cho tất cả SSE clients
// Dùng mutex vì có thể nhiều goroutines truy cập đồng thời
var (
	sseClients = struct {
		mu      sync.RWMutex
		clients map[string]*SSEClient
	}{
		clients: make(map[string]*SSEClient),
	}
)

// addSSEClient: Thêm client mới vào danh sách
func addSSEClient(clientID string) *SSEClient {
	sseClients.mu.Lock()
	defer sseClients.mu.Unlock()

	// Tạo client với buffered channels
	client := &SSEClient{
		ID:    clientID,
		Events: make(chan string, 10),
		Done:   make(chan struct{}),
	}

	sseClients.clients[clientID] = client
	log.Printf("[SSE] Client added: %s (total: %d)", clientID, len(sseClients.clients))
	return client
}

// removeSSEClient: Xóa client khỏi danh sách
func removeSSEClient(clientID string) {
	sseClients.mu.Lock()
	defer sseClients.mu.Unlock()

	if client, exists := sseClients.clients[clientID]; exists {
		close(client.Done) // Signal để stop event loop
		close(client.Events)
		delete(sseClients.clients, clientID)
		log.Printf("[SSE] Client removed: %s (total: %d)", clientID, len(sseClients.clients))
	}
}

// broadcastToAll: Gửi event đến TẤT CẢ clients đang kết nối
// Dùng select với default để non-blocking
func broadcastToAll(message string) {
	sseClients.mu.RLock()
	defer sseClients.mu.RUnlock()

	for _, client := range sseClients.clients {
		select {
		case client.Events <- message:
			// Gửi thành công
		default:
			// Client buffer full (đang xử lý chậm), skip
			log.Printf("[SSE] Client %s buffer full, skipping", client.ID)
		}
	}
}

// --- KHU VỰC 2: SSE Handler ---

// sseHandler: HTTP handler cho SSE endpoint
// Gin hỗ trợ SSE "out of the box" với c.SSEvent()
func sseHandler(c *gin.Context) {
	// Bước 1: Xác định client ID
	clientID := c.Query("client_id")
	if clientID == "" {
		clientID = fmt.Sprintf("client-%s-%d", c.ClientIP(), time.Now().UnixNano())
	}

	// Bước 2: Setup SSE headers
	// Rất quan trọng! Không có headers đúng thì browser không nhận ra SSE
	c.Header("Content-Type", "text/event-stream") // MIME type cho SSE
	c.Header("Cache-Control", "no-cache")         // Không cache
	c.Header("Connection", "keep-alive")           // Giữ connection alive
	c.Header("Access-Control-Allow-Origin", "*")    // Cho phép cross-origin (CORS)

	// Optional: thêm headers để nginx/proxy không buffer
	c.Header("X-Accel-Buffering", "no")

	// Bước 3: Tạo client và đăng ký vào manager
	client := addSSEClient(clientID)
	defer removeSSEClient(clientID) // Cleanup khi disconnect

	log.Printf("[SSE] Client %s connected", clientID)

	// Bước 4: Gửi initial event (để xác nhận connection thành công)
	// Format: c.SSEvent(eventType, data)
	c.SSEvent("connected", fmt.Sprintf(`{"client_id":"%s","message":"Connected to SSE server"}`, clientID))

	// Flush ngay để client nhận được
	c.Writer.Flush()

	// Bước 5: Event loop - liên tục gửi events cho đến khi client disconnect
	//
	// Dùng Gin.Stream() - built-in SSE streaming support
	// Stream nhận 1 callback function với io.Writer, gọi callback liên tục
	// cho đến khi callback trả false (client disconnect)
	c.Stream(func(w io.Writer) bool {
		select {
		case event := <-client.Events:
			// Event từ broadcast (cho tất cả clients)
			c.SSEvent("message", event)
			return true // Tiếp tục streaming

		case <-client.Done:
			// Client disconnect signal
			log.Printf("[SSE] Client %s disconnected", clientID)
			return false // Dừng streaming

		case <-c.Request.Context().Done():
			// HTTP connection closed
			log.Printf("[SSE] Client %s connection closed", clientID)
			return false
		}
	})
}

// --- KHU VỰC 3: HTTP Endpoints ---

// GET /stream/demo - Demo endpoint với auto-generated events
// Gửi counter và random numbers định kỳ
func demoStreamHandler(c *gin.Context) {
	clientID := c.Query("client_id")
	if clientID == "" {
		clientID = fmt.Sprintf("client-%s-%d", c.ClientIP(), time.Now().UnixNano())
	}

	// Setup SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("X-Accel-Buffering", "no")

	// Đăng ký client
	client := addSSEClient(clientID)
	defer removeSSEClient(clientID)

	log.Printf("[SSE] Demo stream started for %s", clientID)

	// Counter demo - gửi số đếm mỗi giây
	counter := 0

	// Ticker để gửi events định kỳ
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	// Stream loop - dùng io.Writer
	c.Stream(func(w io.Writer) bool {
		select {
		case <-ticker.C:
			counter++

			// Gửi counter event
			c.SSEvent("counter", fmt.Sprintf(`{"count":%d}`, counter))

			// Random number mỗi 3 ticks
			if counter%3 == 0 {
				randomNum := rand.Intn(100)
				c.SSEvent("random", fmt.Sprintf(`{"number":%d}`, randomNum))
			}

			// Heartbeat mỗi 10 ticks
			if counter%10 == 0 {
				c.SSEvent("heartbeat", fmt.Sprintf(`{"time":"%s"}`, time.Now().Format(time.RFC3339)))
			}

			return true

		case event := <-client.Events:
			// Event từ broadcast
			c.SSEvent("notification", event)
			return true

		case <-client.Done:
			log.Printf("[SSE] Demo stream ended for %s", clientID)
			return false

		case <-c.Request.Context().Done():
			return false
		}
	})
}

// POST /broadcast - Broadcast event đến tất cả clients
// curl -X POST http://localhost:8080/broadcast?msg=Hello
func broadcastHandler(c *gin.Context) {
	message := c.Query("msg")
	if message == "" {
		message = "Default broadcast message"
	}

	log.Printf("[SSE] Broadcasting: %s", message)
	broadcastToAll(message)

	c.JSON(http.StatusOK, gin.H{
		"status":  "broadcasted",
		"message": message,
		"clients": len(sseClients.clients),
	})
}

// GET /clients - Debug: list connected clients
func listClientsHandler(c *gin.Context) {
	sseClients.mu.RLock()
	defer sseClients.mu.RUnlock()

	clients := make([]string, 0, len(sseClients.clients))
	for id := range sseClients.clients {
		clients = append(clients, id)
	}

	c.JSON(http.StatusOK, gin.H{
		"count":   len(clients),
		"clients": clients,
	})
}

// --- KHU VỰC 4: Background Event Generator (Demo) ---

// startBackgroundEvents: Gửi random events định kỳ (demo)
// Trong production, events sẽ đến từ database, queue, etc.
func startBackgroundEvents() {
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for range ticker.C {
			// Tạo random event
			events := []string{
				"New user registered",
				"Payment received",
				"System backup completed",
				"Cache cleared",
				"New comment posted",
			}
			event := events[rand.Intn(len(events))]

			log.Printf("[BACKGROUND] Broadcasting: %s", event)
			broadcastToAll(event)
		}
	}()
}

// --- KHU VỰC 5: Main ---

func main() {
	// Khởi động background event generator
	startBackgroundEvents()

	// Seed random
	rand.Seed(time.Now().UnixNano())

	// Tạo Gin router
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// Endpoints
	r.GET("/events", sseHandler)             // Basic SSE endpoint
	r.GET("/stream/demo", demoStreamHandler) // Demo với counter
	r.POST("/broadcast", broadcastHandler)   // Broadcast từ HTTP
	r.GET("/clients", listClientsHandler)     // Debug: list clients

	fmt.Println("SSE Streaming Server running on :8080")
	fmt.Println("")
	fmt.Println("Endpoints:")
	fmt.Println("  http://localhost:8080/events          - Basic SSE stream")
	fmt.Println("  http://localhost:8080/stream/demo     - Demo with counter")
	fmt.Println("  http://localhost:8080/broadcast?msg=  - Broadcast from HTTP")
	fmt.Println("")
	fmt.Println("Browser console test:")
	fmt.Println(`  const source = new EventSource("http://localhost:8080/events")`)
	fmt.Println(`  source.onmessage = (e) => console.log("message:", e.data)`)
	fmt.Println(`  source.addEventListener("notification", (e) => console.log("notification:", e.data))`)

	log.Fatal(r.Run(":8080"))
}