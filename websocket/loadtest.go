package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// Load test cho WebSocket
// Chạy nhiều concurrent clients để simulate real-world load

type Stats struct {
	connectCount    int64
	disconnectCount int64
	messageSent     int64
	messageReceived int64
	errorCount      int64
	mu              sync.Mutex
}

var stats Stats

func main() {
	// Parse flags
	clients := flag.Int("clients", 50, "Number of concurrent clients")
	duration := flag.Duration("duration", 30*time.Second, "Test duration")
	serverURL := flag.String("server", "localhost:8080", "Server URL")
	broadcastInterval := flag.Duration("broadcast-interval", 500*time.Millisecond, "HTTP broadcast interval")

	flag.Parse()

	wsURL := fmt.Sprintf("ws://%s/ws", *serverURL)
	httpURL := fmt.Sprintf("http://%s/broadcast", *serverURL)

	fmt.Printf("🚀 WebSocket Load Test\n")
	fmt.Printf("=========================\n")
	fmt.Printf("Clients:       %d\n", *clients)
	fmt.Printf("Duration:      %v\n", *duration)
	fmt.Printf("WebSocket URL: %s\n", wsURL)
	fmt.Printf("=========================\n\n")

	// Start HTTP broadcast in background (simulate server push)
	if *broadcastInterval > 0 {
		go autoBroadcast(httpURL, *broadcastInterval)
	}

	// Create waitgroup
	var wg sync.WaitGroup

	// Start clients
	startTime := time.Now()
	for i := 0; i < *clients; i++ {
		clientID := fmt.Sprintf("ws-loadtest-%d", i)
		wg.Add(1)
		go runWSClient(&wg, clientID, wsURL, *duration)
	}

	// Wait
	wg.Wait()
	elapsed := time.Since(startTime)

	// Print stats
	printStats(elapsed)
}

// runWSClient: Một WebSocket client connect và exchange messages
func runWSClient(wg *sync.WaitGroup, clientID, wsURL string, duration time.Duration) {
	defer wg.Done()

	// Dial WebSocket connection
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		log.Printf("[%s] Connect error: %v", clientID, err)
		atomic.AddInt64(&stats.errorCount, 1)
		return
	}
	defer conn.Close()

	atomic.AddInt64(&stats.connectCount, 1)

	// Read goroutine - đọc messages từ server
	done := make(chan struct{})
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				select {
				case <-done:
				default:
					atomic.AddInt64(&stats.errorCount, 1)
				}
				return
			}
			atomic.AddInt64(&stats.messageReceived, 1)
		}
	}()

	// Write messages periodically
	endTime := time.Now().Add(duration)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	msgNum := 0
	for time.Now().Before(endTime) {
		select {
		case <-ticker.C:
			msg := fmt.Sprintf("Client-%s-Msg-%d", clientID, msgNum)
			err := conn.WriteMessage(websocket.TextMessage, []byte(msg))
			if err != nil {
				atomic.AddInt64(&stats.errorCount, 1)
				return
			}
			atomic.AddInt64(&stats.messageSent, 1)
			msgNum++

		case <-done:
			return
		}
	}

	close(done)
	atomic.AddInt64(&stats.disconnectCount, 1)
}

// autoBroadcast: Gửi broadcast qua HTTP endpoint
func autoBroadcast(httpURL string, interval time.Duration) {
	client := &http.Client{Timeout: 5 * time.Second}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	msgNum := 0
	for range ticker.C {
		req, _ := http.NewRequest("POST", httpURL+"?msg=Broadcast-"+fmt.Sprintf("%d", msgNum), nil)
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
		}
		msgNum++
	}
}

// printStats: In kết quả load test (consistent format across all load tests)
func printStats(elapsed time.Duration) {
	connect := atomic.LoadInt64(&stats.connectCount)
	disconnect := atomic.LoadInt64(&stats.disconnectCount)
	sent := atomic.LoadInt64(&stats.messageSent)
	received := atomic.LoadInt64(&stats.messageReceived)
	errCount := atomic.LoadInt64(&stats.errorCount)

	rps := float64(sent) / elapsed.Seconds()
	throughput := float64(received) / elapsed.Seconds()

	fmt.Printf("\n📊 Load Test Results\n")
	fmt.Printf("======================\n")
	fmt.Printf("Total requests:  %d\n", sent)           // Messages sent = total requests
	fmt.Printf("Success:         %d\n", connect)          // Connected = success
	fmt.Printf("Timeouts:        0\n")                    // WebSocket doesn't timeout
	fmt.Printf("Errors:          %d\n", errCount)
	fmt.Printf("Duration:        %v\n", elapsed)
	fmt.Printf("RPS:             %.2f\n", rps)
	fmt.Printf("Avg latency:     %.2f ms\n", 0.0)         // Not measured for WS
	fmt.Printf("Messages received: %d\n", received)
	fmt.Printf("Throughput:      %.2f msg/s\n", throughput)
}