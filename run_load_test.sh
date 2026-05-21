#!/bin/bash
# ============================================================
# LOAD TEST PIPELINE
# Chạy load test cho tất cả techniques
# Tăng dần clients để tìm giới hạn
# ============================================================

set -e  # Stop on error

# --- Cấu hình ---
BASE_DIR="$(cd "$(dirname "$0")" && pwd)"
RESULTS_FILE="$BASE_DIR/results/load_test_results.md"
TIMESTAMP=$(date '+%Y-%m-%d %H:%M:%S')

# Clients để test (tăng dần)
CLIENT_COUNTS=(5 10 20 30 50 75 100)

# Duration mỗi test (giây)
DURATION="15s"

# Broadcast interval
BROADCAST_INTERVAL="200ms"

# Server port
PORT=8080

# --- Màu cho output ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# --- Functions ---

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[✓]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[!]${NC} $1"
}

log_error() {
    echo -e "${RED}[✗]${NC} $1"
}

# Kiểm tra port có đang được sử dụng không
is_port_in_use() {
    lsof -i :$PORT > /dev/null 2>&1
}

# Kill process trên port
kill_port() {
    if is_port_in_use; then
        log_info "Killing existing process on port $PORT..."
        lsof -ti :$PORT | xargs kill -9 2>/dev/null || true
        sleep 1
    fi
}

# Đợi server ready
wait_for_server() {
    local max_attempts=30
    local attempt=0
    while [ $attempt -lt $max_attempts ]; do
        if curl -s "http://localhost:$PORT/clients" > /dev/null 2>&1; then
            return 0
        fi
        sleep 0.5
        attempt=$((attempt + 1))
    done
    return 1
}

# Chạy load test cho một technique
run_load_test() {
    local tech=$1
    local binary=$2
    local load_test_file=$3
    
    echo ""
    log_info "=========================================="
    log_info "Testing: $tech"
    log_info "=========================================="
    
    # Change to technique directory
    cd "$BASE_DIR/$tech"
    
    # Kill any existing server on port
    kill_port
    
    # Start server in background
    log_info "Starting $tech server..."
    ./$binary &
    SERVER_PID=$!
    
    # Wait for server to be ready
    if ! wait_for_server; then
        log_error "Server failed to start for $tech"
        kill $SERVER_PID 2>/dev/null || true
        return 1
    fi
    
    log_success "Server started (PID: $SERVER_PID)"
    
    # Run tests with different client counts
    for CLIENTS in "${CLIENT_COUNTS[@]}"; do
        echo ""
        log_info "--- Testing $tech with $CLIENTS clients ---"
        
        # Run load test and capture output
        OUTPUT=$(go run $load_test_file \
            -clients=$CLIENTS \
            -duration=$DURATION \
            -broadcast-interval=$BROADCAST_INTERVAL 2>&1)
        
        # Parse output to extract metrics
        echo "$OUTPUT"
        
        # Extract key metrics
        TOTAL_REQ=$(echo "$OUTPUT" | grep "Total requests:" | awk '{print $3}')
        SUCCESS=$(echo "$OUTPUT" | grep "Success:" | awk '{print $2}')
        TIMEOUT=$(echo "$OUTPUT" | grep "Timeouts:" | awk '{print $2}')
        ERRORS=$(echo "$OUTPUT" | grep "Errors:" | awk '{print $2}')
        RPS=$(echo "$OUTPUT" | grep "RPS:" | awk '{print $2}')
        LATENCY=$(echo "$OUTPUT" | grep "Avg latency:" | awk '{print $3}')
        DURATION_VAL=$(echo "$OUTPUT" | grep "Duration:" | awk '{print $2}')
        
        # Write to results file
        echo "| $tech | $CLIENTS | $TOTAL_REQ | $SUCCESS | $TIMEOUT | $ERRORS | $RPS | $LATENCY |" >> "$RESULTS_FILE"
        
        # Check if error rate is too high (stress test indicator)
        if [ ! -z "$ERRORS" ] && [ ! -z "$TOTAL_REQ" ] && [ $TOTAL_REQ -gt 0 ]; then
            ERROR_RATE=$((ERRORS * 100 / TOTAL_REQ))
            if [ $ERROR_RATE -gt 20 ]; then
                log_warn "High error rate ($ERROR_RATE%) - stopping stress test for $tech"
                break
            fi
        fi
        
        # Wait a bit between tests
        sleep 2
    done
    
    # Cleanup
    log_info "Stopping server..."
    kill $SERVER_PID 2>/dev/null || true
    sleep 1
    
    log_success "Completed $tech tests"
}

# ============================================================
# MAIN PIPELINE
# ============================================================

echo ""
log_info "=========================================="
log_info "  LOAD TEST PIPELINE"
log_info "  $(date)"
log_info "=========================================="
echo ""

# Create results directory
mkdir -p "$BASE_DIR/results"

# Initialize results file
cat > "$RESULTS_FILE" << EOF
# Load Test Results

**Generated:** $TIMESTAMP  
**Test Config:** Duration=$DURATION, Broadcast Interval=$BROADCAST_INTERVAL

## Results Summary

| Technique | Clients | Total Req | Success | Timeouts | Errors | RPS | Latency (ms) |
|-----------|---------|-----------|---------|----------|--------|-----|--------------|
EOF

echo "" >> "$RESULTS_FILE"

# Chạy test cho từng technique
run_load_test "long_polling" "long_polling" "load_test.go"
run_load_test "websocket" "websocket" "load_test.go"
run_load_test "streaming" "streaming" "load_test.go"

# Generate summary
echo "" >> "$RESULTS_FILE"

cat >> "$RESULTS_FILE" << 'EOF'

## Analysis

### Long Polling
- Đánh giá scalability
- Xác định điểm giới hạn (thrown errors / timeout tăng)

### WebSocket
- Đánh giá connection handling
- Xác định max concurrent connections

### SSE Streaming
- Đánh giá persistent connection behavior
- Xác định events/second throughput

## Recommendations

- Start với số clients an toàn (50% của max)
- Monitor CPU/RAM khi chạy production
- Implement graceful degradation khi approaching limits

---
*Generated by Load Test Pipeline*
EOF

log_success ""
log_success "=========================================="
log_success "  LOAD TEST COMPLETED!"
log_success "=========================================="
log_success ""
log_success "Results saved to: $RESULTS_FILE"
log_success ""

# Display results
cat "$RESULTS_FILE"

echo ""
log_info "Pipeline completed at $(date)"