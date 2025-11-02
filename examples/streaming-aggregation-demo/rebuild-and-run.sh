#!/bin/bash

set -e

echo "========================================="
echo "Rebuild and Run Streaming Aggregation Demo"
echo "========================================="
echo ""

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check prerequisites
echo "Checking prerequisites..."
if ! command -v docker &> /dev/null; then
    echo -e "${RED}❌ Docker is not installed. Please install Docker first.${NC}"
    exit 1
fi

if ! command -v docker-compose &> /dev/null; then
    echo -e "${RED}❌ Docker Compose is not installed. Please install Docker Compose first.${NC}"
    exit 1
fi

if ! command -v go &> /dev/null; then
    echo -e "${RED}❌ Go is not installed. Please install Go first.${NC}"
    exit 1
fi

if ! command -v make &> /dev/null; then
    echo -e "${RED}❌ Make is not installed. Please install Make first.${NC}"
    exit 1
fi

echo -e "${GREEN}✅ All prerequisites met${NC}"

# Get the repository root (two levels up from demo directory)
REPO_ROOT=$(cd "$(dirname "$0")/../.." && pwd)
DEMO_DIR="$REPO_ROOT/examples/streaming-aggregation-demo"

echo ""
echo "Building OpenTelemetry Collector with streaming aggregation processor..."
echo "========================================="

# Build the collector
cd "$REPO_ROOT"
echo "Generating collector code..."
make genotelcontribcol

echo "Compiling collector binary for Linux/AMD64..."
cd cmd/otelcontribcol
GOOS=linux GOARCH=amd64 go build -o "$DEMO_DIR/otelcol-streaming" .

if [ ! -f "$DEMO_DIR/otelcol-streaming" ]; then
    echo -e "${RED}❌ Failed to build collector binary${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Collector binary built successfully${NC}"

# Return to demo directory
cd "$DEMO_DIR"

echo ""
echo "Stopping existing services and resetting Prometheus volume..."
echo "========================================="
docker-compose down 2>/dev/null || true

# Reset only the Prometheus volume for fresh data
echo "Resetting Prometheus volume for fresh data..."
docker volume rm streaming-aggregation-demo_prometheus-data 2>/dev/null || true
echo "Prometheus volume reset complete"

echo ""
echo "Starting Docker Compose services..."
echo "========================================="
docker-compose up -d --build

echo ""
echo "Waiting for services to be ready..."
for i in {1..10}; do
    echo -n "."
    sleep 1
done
echo ""

# Check if services are running
echo ""
echo "Service Status:"
echo "========================================="
docker-compose ps

# Check if all services are running
RUNNING_COUNT=$(docker-compose ps | grep -c "Up" || true)
if [ "$RUNNING_COUNT" -lt 5 ]; then
    echo -e "${YELLOW}⚠️  Some services may not be running properly${NC}"
    echo "Check logs with: docker-compose logs"
else
    echo -e "${GREEN}✅ All services are running${NC}"
fi

echo ""
echo -e "${GREEN}🎉 Demo is running!${NC}"
echo "========================================="
echo ""
echo "📊 Grafana Dashboards: http://localhost:3001"
echo "   Username: admin"
echo "   Password: admin"
echo "   Available dashboards:"
echo "   - Histogram Comparison Dashboard"
echo "   - Streaming Aggregation Verbose"
echo ""
echo "📈 Prometheus: http://localhost:9091"
echo ""
echo "🔍 Collector Metrics:"
echo "   Raw (no aggregation): http://localhost:8893/metrics"
echo "   Aggregated: http://localhost:8891/metrics"
echo ""
echo "📝 Useful commands:"
echo "  View logs:           docker-compose logs -f [service-name]"
echo "  View all logs:       docker-compose logs -f"
echo "  Stop the demo:       docker-compose down"
echo "  Clean up everything: docker-compose down -v"
echo "  Rebuild only:        $0"
echo ""
echo "Service names: prometheus, grafana, collector-raw, collector-aggregated, metric-generator"
echo ""
