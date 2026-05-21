package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// ============================================================
// WEBSOCKET - Chi tiết từng phần
// ============================================================
// WebSocket khác Long Polling ở chỗ:
//   1. Connection được UPGRADE từ HTTP (101 Switching Protocols)
//   2. Sau handshake, cả client và server đều gửi data TỰ DO
//   3. Không có request/response cycle - data có thể gửi bất kỳ lúc nào
//   4. Frame-based (rất nhẹ so với HTTP headers)
//
// Điểm quan trọng:
//   - Stateful: phải track connection
//   - Cần heartbeat để phát hiện connection chết
//   - Cần lock khi broadcast vì nhiều goroutines gửi cùng lúc
// ============================================================

// --- KHU VỰC 1: Cấu hình ---

// upgrader: Cấu hình để upgrade HTTP connection thành WebSocket
// Đây là "cánh cổng" từ HTTP sang WebSocket
var upgrader = websocket.Upgrader{
	// CheckOrigin: kiểm tra origin của request
	// Trong production nên validate origin để tránh CSWSH
	// (Cross-Site WebSocket Hijacking)
	CheckOrigin: func(r *http.Request) bool {
		return true // Cho phép tất cả (demo only!)
		// Production nên:
		// return r.Header.Get("Origin") == "https://yourdomain.com"
	},

	// ReadBufferSize, WriteBufferSize: buffer cho frame I/O
	// Ảnh hưởng đến memory usage và performance
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// pingInterval: Khoảng thời gian gửi ping để giữ connection alive
// WebSocket có built-in ping/pong mechanism
const pingInterval = 30 * time.Second

// pingWait: Thời gian chờ pong response
// Nếu không nhận được pong trong thời gian này → coi như disconnect
const pingWait = 60 * time.Second

// --- KHU VỰC 2: Client Hub (Broadcast Manager) ---

// Client: đại diện cho 1 WebSocket connection
type Client struct {
	// conn: WebSocket connection của client này
	conn *websocket.Conn

	// send: Channel dùng để gửi message đến client
	// Tại sao dùng channel thay vì gửi trực tiếp?
	// Vì goroutine đọc message từ connection (readLoop)
	// cần giao tiếp với goroutine gửi message (writeLoop)
	// Channel là cách an toàn để sync giữa 2 goroutines
	send chan []byte

	// ID để identify client (debugging)
	ID string
}

// Hub: Quản lý tất cả clients đang kết nối
// Central "bộ điều phối" cho tất cả WebSocket connections
type Hub struct {
	// clients: Map tất cả clients, key là client ID
	// Dùng map để lookup nhanh O(1) khi cần broadcast
	clients map[*Client]bool

	// broadcast: Channel nhận message để gửi đến tất cả clients
	// Ai muốn broadcast message → gửi vào channel này
	broadcast chan []byte

	// register: Channel để đăng ký client mới
	// Khi có client kết nối → gửi pointer của Client vào đây
	register chan *Client

	// unregister: Channel để hủy đăng ký client
	// Khi client disconnect → gửi pointer vào đây
	unregister chan *Client

	// Mutex bảo vệ clients map (vì nhiều goroutines truy cập)
	mu sync.RWMutex
}

// Global hub instance
var hub = &Hub{
	clients:    make(map[*Client]bool),
	broadcast:  make(chan []byte, 256), // buffered để không block sender
	register:   make(chan *Client),
	unregister: make(chan *Client),
}

// Run: Main loop của Hub - chạy trong 1 goroutine riêng
// Lắng nghe các channels: register, unregister, broadcast
//
// Đây là "event loop" của WebSocket server
// Tất cả modifications vào clients map đều diễn ra ở đây
// để đảm bảo thread-safety
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			// Có client mới kết nối
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("[WS] Client registered: %s (total: %d)", client.ID, len(h.clients))

		case client := <-h.unregister:
			// Có client ngắt kết nối
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send) // Đóng send channel
			}
			h.mu.Unlock()
			log.Printf("[WS] Client unregistered: %s (total: %d)", client.ID, len(h.clients))

		case message := <-h.broadcast:
			// Có message cần broadcast đến tất cả clients
			h.mu.RLock()
			for client := range h.clients {
				// Non-blocking send
				// Nếu channel full (client quá chậm) → bỏ qua client đó
				select {
				case client.send <- message:
				default:
					// Client send buffer full, skip
					// (client đang xử lý chậm hoặc disconnect)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// --- KHU VỰC 3: Connection Handler ---

// wsHandler: HTTP handler cho WebSocket upgrade
// Gin gọi handler này khi client truy cập /ws
func wsHandler(c *gin.Context) {
	// Bước 1: Upgrade HTTP → WebSocket
	// Đây là nơi "101 Switching Protocols" xảy ra
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[WS] Upgrade failed: %v", err)
		return
	}

	// Bước 2: Tạo client object
	// Dùng IP làm ID (có thể dùng user ID nếu có auth)
	client := &Client{
		conn: conn,
		send: make(chan []byte, 256), // buffered
		ID:   fmt.Sprintf("%s-%d", c.ClientIP(), time.Now().UnixNano()),
	}

	// Bước 3: Đăng ký client vào hub
	hub.register <- client

	// Bước 4: Khởi động read và write goroutines
	// Đọc và gửi chạy độc lập để không blocking nhau
	go client.writePump() // Gửi message cho client
	go client.readPump() // Đọc message từ client

	// Lưu ý: Khi function này return (client disconnect),
	// hub.unregister sẽ được gọi trong readPump
}

// readPump: Đọc messages từ WebSocket connection
// Chạy trong goroutine riêng
func (c *Client) readPump() {
	defer func() {
		// Khi readPump kết thúc (error hoặc client close),
		// cleanup: hủy đăng ký khỏi hub, đóng connection
		hub.unregister <- c
		c.conn.Close()
	}()

	// Cấu hình read deadline
	// Cần thiết vì TCP connection có thể "hang" mãi mãi
	// nếu peer không bao giờ gửi data
	c.conn.SetReadDeadline(time.Now().Add(pingWait))

	// Set pong handler - khi server nhận được pong từ client
	// → reset read deadline (connection còn sống)
	c.conn.SetPongHandler(func(appData string) error {
		c.conn.SetReadDeadline(time.Now().Add(pingWait))
		return nil
	})

	// Read loop
	for {
		// Đọc message từ connection
		// MessageType có thể là:
		//   - websocket.TextMessage
		//   - websocket.BinaryMessage
		//   - websocket.CloseMessage
		//   - websocket.PingMessage
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			// Lỗi có thể là:
			// - Client disconnect bình thường (EOF)
			// - WebSocket close frame
			// - Read timeout
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[WS] Read error: %v", err)
			}
			break // Exit loop, defer sẽ cleanup
		}

		// Log message nhận được
		log.Printf("[WS] Message from %s: %s", c.ID, string(message))

		// Echo back cho tất cả clients (broadcast)
		// Format: "[ClientID] message"
		broadcastMsg := fmt.Sprintf("[%s] %s", c.ID, string(message))
		hub.broadcast <- []byte(broadcastMsg)
	}
}

// writePump: Gửi messages từ hub → client
// Chạy trong goroutine riêng
func (c *Client) writePump() {
	// Tạo ticker để gửi ping định kỳ
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		c.conn.Close() // Đóng connection khi writePump kết thúc
	}()

	for {
		select {
		case message, ok := <-c.send:
			// Có message cần gửi cho client
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

			// Nếu channel đã đóng (client unregistered) → thoát
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			// Gửi message
			// WriteMessage gửi 1 message hoàn chỉnh
			// (không phải frame - WebSocket library tự chia frame)
			err := c.conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Printf("[WS] Write error: %v", err)
				return
			}

			// Optional: log nếu queue đang filling up
			// (nếu len(c.send) > 200 → client đang xử lý chậm)

		case <-ticker.C:
			// Timer tick → gửi ping để giữ connection alive
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

			// WriteControl gửi control frame (ping/pong/close)
			// Control frames có priority cao hơn data frames
			err := c.conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(10*time.Second))
			if err != nil {
				log.Printf("[WS] Ping failed: %v", err)
				return
			}
		}
	}
}

// --- KHU VỰC 4: HTTP Endpoints ---

// GET /broadcast - Broadcast message lên tất cả WebSocket clients
// Dùng để test từ browser hoặc curl
func broadcastHandler(c *gin.Context) {
	message := c.Query("msg")
	if message == "" {
		message = "Hello from HTTP!"
	}

	log.Printf("[HTTP] Broadcasting: %s", message)
	hub.broadcast <- []byte(message)

	c.JSON(http.StatusOK, gin.H{
		"status":  "broadcasted",
		"message": message,
		"clients": len(hub.clients),
	})
}

// GET /clients - Debug: list all connected clients
func listClientsHandler(c *gin.Context) {
	hub.mu.RLock()
	defer hub.mu.RUnlock()

	clients := make([]string, 0, len(hub.clients))
	for client := range hub.clients {
		clients = append(clients, client.ID)
	}

	c.JSON(http.StatusOK, gin.H{
		"count":   len(clients),
		"clients": clients,
	})
}

// --- KHU VỰC 5: Main ---

func main() {
	// Khởi động Hub's event loop (trong goroutine riêng)
	// Đây là "trái tim" của WebSocket server
	go hub.Run()

	// Tạo Gin router
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// Endpoints
	r.GET("/ws", wsHandler)               // WebSocket endpoint
	r.GET("/broadcast", broadcastHandler) // HTTP endpoint để trigger broadcast
	r.GET("/clients", listClientsHandler)

	fmt.Println("WebSocket Server running on :8080")
	fmt.Println("Endpoints:")
	fmt.Println("  ws://localhost:8080/ws                   - WebSocket connection")
	fmt.Println("  http://localhost:8080/broadcast?msg=Hello  - Broadcast from HTTP")
	fmt.Println("")
	fmt.Println("Browser console test:")
	fmt.Println(`  ws = new WebSocket("ws://localhost:8080/ws")`)
	fmt.Println(`  ws.onmessage = (e) => console.log(e.data)`)
	fmt.Println(`  ws.send("Hello!")`)

	log.Fatal(r.Run(":8080"))
}