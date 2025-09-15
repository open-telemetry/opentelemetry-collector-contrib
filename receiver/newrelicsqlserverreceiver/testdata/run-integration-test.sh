#!/bin/bash

# Test script for New Relic SQL Server receiver
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TESTDATA_DIR="${SCRIPT_DIR}/testdata"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}Starting New Relic SQL Server receiver integration tests...${NC}"

# Function to cleanup
cleanup() {
    echo -e "${YELLOW}Cleaning up test environment...${NC}"
    cd "${TESTDATA_DIR}"
    docker-compose down -v
}

# Set trap to cleanup on exit
trap cleanup EXIT

# Change to testdata directory
cd "${TESTDATA_DIR}"

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo -e "${RED}Error: Docker is not running. Please start Docker and try again.${NC}"
    exit 1
fi

# Start SQL Server and wait for it to be ready
echo -e "${YELLOW}Starting SQL Server container...${NC}"
docker-compose up -d sqlserver

# Wait for SQL Server to be healthy
echo -e "${YELLOW}Waiting for SQL Server to be ready...${NC}"
timeout=60
counter=0
while ! docker-compose exec -T sqlserver /opt/mssql-tools/bin/sqlcmd -S localhost -U sa -P TestPassword123! -Q "SELECT 1" > /dev/null 2>&1; do
    if [ $counter -ge $timeout ]; then
        echo -e "${RED}Timeout waiting for SQL Server to be ready${NC}"
        exit 1
    fi
    echo "Waiting for SQL Server... ($counter/$timeout)"
    sleep 1
    counter=$((counter + 1))
done

echo -e "${GREEN}SQL Server is ready!${NC}"

# Run SQL scripts to set up test data
echo -e "${YELLOW}Setting up test data...${NC}"
for sql_file in sql-scripts/*.sql; do
    if [ -f "$sql_file" ]; then
        echo "Executing $sql_file..."
        docker-compose exec -T sqlserver /opt/mssql-tools/bin/sqlcmd -S localhost -U sa -P TestPassword123! -i "/docker-entrypoint-initdb.d/$(basename "$sql_file")"
    fi
done

# Build the custom collector with the new receiver
echo -e "${YELLOW}Building custom OpenTelemetry Collector...${NC}"
cd "${SCRIPT_DIR}"

# Create a temporary builder config
cat > builder-config.yaml << EOF
dist:
  name: otelcol-dev
  description: Basic OTel Collector distribution for Developers
  output_path: ./bin

receivers:
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver v0.0.0
    path: ./

processors:
  - gomod: go.opentelemetry.io/collector/processor/batchprocessor v0.87.0
  - gomod: go.opentelemetry.io/collector/processor/memorylimiterprocessor v0.87.0

exporters:
  - gomod: go.opentelemetry.io/collector/exporter/debugexporter v0.87.0
  - gomod: go.opentelemetry.io/collector/exporter/otlpexporter v0.87.0

connectors:
  - gomod: go.opentelemetry.io/collector/connector/forwardconnector v0.87.0
EOF

# Build the collector (if ocb is available)
if command -v ocb >/dev/null 2>&1; then
    echo -e "${YELLOW}Using OpenTelemetry Collector Builder to create custom collector...${NC}"
    ocb --config builder-config.yaml
    COLLECTOR_BINARY="./bin/otelcol-dev"
else
    echo -e "${YELLOW}OpenTelemetry Collector Builder not found. Using pre-built collector...${NC}"
    COLLECTOR_BINARY="otelcol-contrib"
fi

# Test the collector configuration
echo -e "${YELLOW}Testing collector configuration...${NC}"
cd "${TESTDATA_DIR}"

# Set environment variables
export SQLSERVER_PASSWORD="TestPassword123!"

# Validate the configuration
if [ -f "${COLLECTOR_BINARY}" ]; then
    "${COLLECTOR_BINARY}" --config-file=config.yaml --dry-run
else
    echo -e "${YELLOW}Custom collector not found, testing with docker...${NC}"
fi

# Start the full stack for integration testing
echo -e "${YELLOW}Starting full test stack...${NC}"
docker-compose up -d

# Wait a bit for metrics collection
echo -e "${YELLOW}Collecting metrics for 30 seconds...${NC}"
sleep 30

# Check if metrics are being collected
echo -e "${YELLOW}Checking collector logs...${NC}"
docker-compose logs otel-collector | grep -i "newrelicsqlserver\|metrics\|error" | tail -20

# Test different scenarios
echo -e "${YELLOW}Running test scenarios...${NC}"

# Test 1: Check basic connectivity
echo "Test 1: Basic connectivity test"
docker-compose exec -T sqlserver /opt/mssql-tools/bin/sqlcmd -S localhost -U sa -P TestPassword123! -Q "SELECT @@VERSION"

# Test 2: Generate some activity
echo "Test 2: Generating database activity"
docker-compose exec -T sqlserver /opt/mssql-tools/bin/sqlcmd -S localhost -U sa -P TestPassword123! -d TestDatabase -Q "EXEC GenerateTestActivity"

# Test 3: Check metrics in logs
echo "Test 3: Checking for specific metrics in logs"
sleep 10
if docker-compose logs otel-collector | grep -q "sqlserver.stats.connections"; then
    echo -e "${GREEN}✓ User connections metric found${NC}"
else
    echo -e "${RED}✗ User connections metric not found${NC}"
fi

echo -e "${GREEN}Integration tests completed!${NC}"
echo -e "${YELLOW}To view Jaeger UI, open: http://localhost:16686${NC}"
echo -e "${YELLOW}To stop the test environment, run: docker-compose down -v${NC}"

# Keep the environment running for manual testing
read -p "Press Enter to stop the test environment, or Ctrl+C to keep it running..."
