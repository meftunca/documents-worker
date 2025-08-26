#!/bin/bash

# Test script for documents-worker
# This script tests the dynamic worker scaling and graceful shutdown

echo "🚀 Starting Documents Worker Test"

# Test configuration
BASE_URL="http://localhost:3001"
API_URL="$BASE_URL/api/v1"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Functions
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if server is running
check_health() {
    print_status "Checking server health..."
    response=$(curl -s -o /dev/null -w "%{http_code}" "$BASE_URL/health")
    if [ "$response" -eq 200 ]; then
        print_success "Server is healthy"
        return 0
    else
        print_error "Server is not responding (HTTP $response)"
        return 1
    fi
}

# Get worker statistics
get_worker_stats() {
    print_status "Getting worker statistics..."
    curl -s "$API_URL/worker/stats" | jq '.' 2>/dev/null || {
        print_warning "Could not parse worker stats JSON"
        curl -s "$API_URL/worker/stats"
    }
}

# Get queue statistics
get_queue_stats() {
    print_status "Getting queue statistics..."
    curl -s "$API_URL/queue/stats" | jq '.' 2>/dev/null || {
        print_warning "Could not parse queue stats JSON"
        curl -s "$API_URL/queue/stats"
    }
}

# Submit test jobs to trigger scaling
submit_test_jobs() {
    local count=${1:-5}
    print_status "Submitting $count test jobs..."
    
    for i in $(seq 1 $count); do
        if [ -f "./test_files/test.pdf" ]; then
            response=$(curl -s -X POST -F "file=@./test_files/test.pdf" "$API_URL/process/document")
            job_id=$(echo $response | jq -r '.job_id' 2>/dev/null)
            if [ "$job_id" != "null" ] && [ -n "$job_id" ]; then
                print_success "Submitted job $i: $job_id"
            else
                print_warning "Job $i submission response: $response"
            fi
        else
            print_warning "test.pdf not found, skipping job $i"
        fi
        sleep 1
    done
}

# Test text extraction
test_text_extraction() {
    print_status "Testing text extraction..."
    if [ -f "./test_files/test.pdf" ]; then
        response=$(curl -s -X POST -F "file=@./test_files/test.pdf" "$API_URL/extract/text")
        echo $response | jq '.' 2>/dev/null || echo $response
    else
        print_warning "test.pdf not found, skipping text extraction test"
    fi
}

# Test image processing
test_image_processing() {
    print_status "Testing image processing..."
    if [ -f "./test_files/test.webp" ]; then
        response=$(curl -s -X POST -F "file=@./test_files/test.webp" "$API_URL/process/image")
        job_id=$(echo $response | jq -r '.job_id' 2>/dev/null)
        if [ "$job_id" != "null" ] && [ -n "$job_id" ]; then
            print_success "Image processing job submitted: $job_id"
        else
            print_warning "Image processing response: $response"
        fi
    else
        print_warning "test.webp not found, skipping image processing test"
    fi
}

# Monitor scaling behavior
monitor_scaling() {
    local duration=${1:-60}
    print_status "Monitoring worker scaling for $duration seconds..."
    
    local end_time=$(($(date +%s) + duration))
    while [ $(date +%s) -lt $end_time ]; do
        echo ""
        echo "=== $(date) ==="
        get_worker_stats
        echo ""
        get_queue_stats
        echo ""
        sleep 10
    done
}

# Test graceful shutdown
test_graceful_shutdown() {
    print_status "Testing graceful shutdown..."
    print_warning "You should manually send SIGTERM to test graceful shutdown"
    print_warning "Run: docker-compose kill -s SIGTERM documents-worker"
    print_warning "Or if running locally: kill -TERM <pid>"
}

# Main test sequence
main() {
    echo "=========================================="
    echo "  Documents Worker Dynamic Scaling Test  "
    echo "=========================================="
    echo ""
    
    # Basic health check
    if ! check_health; then
        print_error "Server is not running. Start it first with:"
        print_error "  docker-compose up -d"
        print_error "  or go run main.go"
        exit 1
    fi
    
    echo ""
    print_status "Initial system state:"
    get_worker_stats
    echo ""
    get_queue_stats
    echo ""
    
    # Test basic functionality
    test_text_extraction
    echo ""
    test_image_processing
    echo ""
    
    # Submit multiple jobs to trigger scaling
    submit_test_jobs 10
    echo ""
    
    # Monitor scaling behavior
    monitor_scaling 30
    
    # Test recommendations
    echo ""
    echo "=========================================="
    echo "  Manual Test Recommendations            "
    echo "=========================================="
    echo ""
    print_status "1. Monitor logs to see worker scaling:"
    echo "  docker-compose logs -f documents-worker"
    echo ""
    print_status "2. Submit more jobs to trigger scale-up:"
    echo "  for i in {1..20}; do curl -X POST -F \"file=@./test_files/test.pdf\" $API_URL/process/document; done"
    echo ""
    print_status "3. Test graceful shutdown:"
    echo "  docker-compose kill -s SIGTERM documents-worker"
    echo ""
    print_status "4. Check worker stats endpoint:"
    echo "  curl $API_URL/worker/stats | jq"
    echo ""
    print_status "5. Check queue stats endpoint:"
    echo "  curl $API_URL/queue/stats | jq"
    echo ""
    
    print_success "Test completed! Check the logs and try manual tests above."
}

# Run main function
main "$@"
