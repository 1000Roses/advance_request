package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Load test cho SSE Streaming
// Chạy nhiều concurrent clients để simulate real-world load

type Stats struct {
	connectCount    int64
	disconnectCount int64
	eventsReceived  int64
	errorCount      int64
	mu              sync.Mutex
}

var stats Stats

func main() {
	// Parse flags
	clients := flag.Int("clients", 50, "Number of concurrent clients")
	duration := flag.Duration("duration", 30*time.Second, "Test duration")
	serverURL := flag.String("server", "http://localhost:8080", "Server URL")
	eventsPath := flag.String("events-path", "/events", "SSE endpoint path")
	broadcastInterval := flag.Duration("broadcast-interval", 500*time.Millisecond, "Broadcast interval")

	flag.Parse()

	eventsURL := *serverURL + *eventsPath
	broadcastURL := *serverURL + "/broadcast"

	fmt.Printf("🚀 SSE Streaming Load Test\n")
	fmt.Printf("==========================\n")
	fmt.Printf("Clients:       %d\n", *clients)
	fmt.Printf("Duration:      %v\n", *duration)
	fmt.Printf("Events URL:    %s\n", eventsURL)
	fmt.Printf("==========================\n\n")

	// Start HTTP broadcast in background (simulate server push)
	if *broadcastInterval > 0 {
		go autoBroadcast(broadcastURL, *broadcastInterval)
	}

	// Create waitgroup
	var wg sync.WaitGroup

	// Start clients
	startTime := time.Now()
	for i := 0; i < *clients; i++ {
		clientID := fmt.Sprintf("sse-loadtest-%d", i)
		wg.Add(1)
		go runSSEClient(&wg, clientID, eventsURL, *duration)
	}

	// Wait
	wg.Wait()
	elapsed := time.Since(startTime)

	// Print stats
	printStats(elapsed)
}

// runSSEClient: Một SSE client connect và nhận events
func runSSEClient(wg *sync.WaitGroup, clientID, eventsURL string, duration time.Duration) {
	defer wg.Done()

	// Tạo HTTP request với custom Client để kiểm soát timeout
	client := &http.Client{
		Timeout: duration + 10*time.Second,
	}

	req, err := http.NewRequest("GET", eventsURL+"?client_id="+clientID, nil)
	if err != nil {
		log.Printf("[%s] Request create error: %v", clientID, err)
		atomic.AddInt64(&stats.errorCount, 1)
		return
	}

	// Important: SSE requires these headers
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[%s] GET error: %v", clientID, err)
		atomic.AddInt64(&stats.errorCount, 1)
		return
	}
	defer resp.Body.Close()

	// Kiểm tra content-type - dùng HasPrefix vì server có thể trả thêm charset
	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/event-stream") {
		log.Printf("[%s] Wrong content type: %s", clientID, contentType)
		atomic.AddInt64(&stats.errorCount, 1)
		return
	}

	atomic.AddInt64(&stats.connectCount, 1)

	// Đọc SSE stream
	// SSE format:
	//   event: message\n
	//   data: {"event":"..."}\n\n
	reader := NewSSEReader(resp.Body)
	endTime := time.Now().Add(duration)

	for time.Now().Before(endTime) {
		event, err := reader.ReadEvent()
		if err != nil {
			if err == io.EOF {
				break
			}
			atomic.AddInt64(&stats.errorCount, 1)
			break
		}

		if event != nil {
			atomic.AddInt64(&stats.eventsReceived, 1)
		}
	}

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

// SSEReader: Simple reader để parse SSE events
type SSEReader struct {
	reader io.Reader
	buffer []byte
}

func NewSSEReader(r io.Reader) *SSEReader {
	return &SSEReader{
		reader: r,
		buffer: make([]byte, 0, 4096),
	}
}

func (s *SSEReader) ReadEvent() ([]byte, error) {
	// Read until we get a complete event (ends with \n\n)
	for {
		// Try to find \n\n in buffer
		for i := 0; i < len(s.buffer)-1; i++ {
			if s.buffer[i] == '\n' && s.buffer[i+1] == '\n' {
				// Found complete event
				event := s.buffer[:i]
				s.buffer = s.buffer[i+2:]
				return event, nil
			}
		}

		// Read more data
		buf := make([]byte, 1024)
		n, err := s.reader.Read(buf)
		if err != nil {
			if len(s.buffer) > 0 {
				// Return remaining buffer as last event
				event := s.buffer
				s.buffer = s.buffer[:0]
				return event, nil
			}
			return nil, err
		}
		s.buffer = append(s.buffer, buf[:n]...)
	}
}

// printStats: In kết quả load test
func printStats(elapsed time.Duration) {
	connect := atomic.LoadInt64(&stats.connectCount)
	disconnect := atomic.LoadInt64(&stats.disconnectCount)
	events := atomic.LoadInt64(&stats.eventsReceived)
	errCount := atomic.LoadInt64(&stats.errorCount)

	eventsPerSec := float64(events) / elapsed.Seconds()
	var eventsPerClient float64
	if connect > 0 {
		eventsPerClient = float64(events) / float64(connect)
	}

	fmt.Printf("\n📊 SSE Streaming Load Test Results\n")
	fmt.Printf("==================================\n")
	fmt.Printf("Connected clients:    %d\n", connect)
	fmt.Printf("Disconnected clients: %d\n", disconnect)
	fmt.Printf("Total events:         %d\n", events)
	fmt.Printf("Errors:               %d\n", errCount)
	fmt.Printf("Duration:             %v\n", elapsed)
	fmt.Printf("Events/second:        %.2f\n", eventsPerSec)
	if connect > 0 {
		fmt.Printf("Events/client:        %.2f (avg)\n", eventsPerClient)
	}
}