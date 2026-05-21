package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Load test cho Long Polling
// Chạy nhiều concurrent clients để simulate real-world load

type Stats struct {
	totalRequests int64
	successCount  int64
	timeoutCount  int64
	errorCount    int64
	totalLatency  int64 // nanoseconds
	mu            sync.Mutex
}

var stats Stats

func main() {
	// Parse flags
	clients := flag.Int("clients", 50, "Number of concurrent clients")
	duration := flag.Duration("duration", 30*time.Second, "Test duration")
	serverURL := flag.String("server", "http://localhost:8080", "Server URL")
	pollURL := flag.String("poll-url", "/poll", "Poll endpoint path")
	broadcastURL := flag.String("broadcast-url", "/publish", "Broadcast endpoint path")
	interval := flag.Duration("broadcast-interval", 500*time.Millisecond, "Broadcast interval")

	flag.Parse()

	fullPollURL := *serverURL + *pollURL
	fullBroadcastURL := *serverURL + *broadcastURL

	fmt.Printf("🚀 Long Polling Load Test\n")
	fmt.Printf("=========================\n")
	fmt.Printf("Clients:     %d\n", *clients)
	fmt.Printf("Duration:    %v\n", *duration)
	fmt.Printf("Poll URL:    %s\n", fullPollURL)
	fmt.Printf("=========================\n\n")

	// Start broadcasting in background (simulate real data)
	if *interval > 0 {
		go autoBroadcast(fullBroadcastURL, *interval)
	}

	// Create waitgroup để đợi tất cả clients
	var wg sync.WaitGroup

	// Start clients
	startTime := time.Now()
	for i := 0; i < *clients; i++ {
		clientID := fmt.Sprintf("loadtest-client-%d", i)
		wg.Add(1)
		go runClient(&wg, clientID, fullPollURL, *duration)
	}

	// Wait for all clients
	wg.Wait()
	elapsed := time.Since(startTime)

	// Print stats
	printStats(elapsed)
}

// runClient: Một client liên tục poll cho đến khi duration hết
func runClient(wg *sync.WaitGroup, clientID, pollURL string, duration time.Duration) {
	defer wg.Done()

	client := &http.Client{
		Timeout: 70 * time.Second, // Slightly > server timeout (60s)
	}

	endTime := time.Now().Add(duration)

	for time.Now().Before(endTime) {
		reqStart := time.Now()

		// Make poll request
		req, _ := http.NewRequest("GET", pollURL+"?client_id="+clientID, nil)
		resp, err := client.Do(req)

		latency := time.Since(reqStart).Nanoseconds()
		atomic.AddInt64(&stats.totalRequests, 1)
		atomic.AddInt64(&stats.totalLatency, latency)

		if err != nil {
			atomic.AddInt64(&stats.errorCount, 1)
			log.Printf("[%s] Error: %v", clientID, err)
			time.Sleep(1 * time.Second) // Wait before retry
			continue
		}

		// Read response body
		// In real test, you'd parse the JSON to check if received=true
		buf := make([]byte, 1024)
		resp.Body.Read(buf)
		resp.Body.Close()

		// Check if received event or timeout
		// For load test, we just count success
		atomic.AddInt64(&stats.successCount, 1)

		// Small delay để tránh hammering
		time.Sleep(10 * time.Millisecond)
	}
}

// autoBroadcast: Gửi broadcast liên tục để simulate real data
func autoBroadcast(url string, interval time.Duration) {
	client := &http.Client{Timeout: 5 * time.Second}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	msgNum := 0
	for range ticker.C {
		msg := fmt.Sprintf("LoadTest-Msg-%d", msgNum)
		resp, err := client.Post(url, "application/x-www-form-urlencoded",
			strings.NewReader("message="+msg))
		if err == nil {
			resp.Body.Close()
		}
		msgNum++
	}
}

// printStats: In kết quả load test
func printStats(elapsed time.Duration) {
	total := atomic.LoadInt64(&stats.totalRequests)
	success := atomic.LoadInt64(&stats.successCount)
	timeout := atomic.LoadInt64(&stats.timeoutCount)
	errCount := atomic.LoadInt64(&stats.errorCount)
	totalLatency := atomic.LoadInt64(&stats.totalLatency)

	rps := float64(total) / elapsed.Seconds()
	var avgLatency float64
	if total > 0 {
		avgLatency = float64(totalLatency) / float64(total) / 1e6 // Convert to ms
	}

	fmt.Printf("\n📊 Load Test Results\n")
	fmt.Printf("======================\n")
	fmt.Printf("Total requests:  %d\n", total)
	fmt.Printf("Success:         %d\n", success)
	fmt.Printf("Timeouts:        %d\n", timeout)
	fmt.Printf("Errors:          %d\n", errCount)
	fmt.Printf("Duration:        %v\n", elapsed)
	fmt.Printf("RPS:             %.2f\n", rps)
	fmt.Printf("Avg latency:     %.2f ms\n", avgLatency)
	if total > 0 {
		fmt.Printf("Error rate:      %.2f%%\n", float64(errCount)/float64(total)*100)
	}
}