#!/bin/bash

# Test script for New Relic SQL Server receiver
set -e

echo "🚀 Starting New Relic SQL Server Receiver Test"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if we're in the right directory
if [[ ! -f "factory.go" ]]; then
    log_error "Please run this script from the newrelicsqlserverreceiver directory"
    exit 1
fi

# Step 1: Build the receiver
log_info "Building the receiver..."
go build .
if [[ $? -eq 0 ]]; then
    log_info "✅ Build successful"
else
    log_error "❌ Build failed"
    exit 1
fi

# Step 2: Run tests
log_info "Running tests..."
go test -v .
if [[ $? -eq 0 ]]; then
    log_info "✅ Tests passed"
else
    log_error "❌ Tests failed"
    exit 1
fi

# Step 3: Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    log_error "Docker is not running. Please start Docker first."
    exit 1
fi

# Step 4: Start SQL Server and OTEL backend
log_info "Starting SQL Server and OTEL backend..."
docker-compose -f docker-compose.test.yml up -d

# Wait for SQL Server to be ready
log_info "Waiting for SQL Server to be ready..."
timeout=60
counter=0
while ! docker exec sqlserver-test /opt/mssql-tools/bin/sqlcmd -S localhost -U sa -P "YourStrong!Passw0rd" -Q "SELECT 1" > /dev/null 2>&1; do
    if [[ $counter -ge $timeout ]]; then
        log_error "SQL Server failed to start within $timeout seconds"
        docker-compose -f docker-compose.test.yml logs sqlserver
        exit 1
    fi
    echo -n "."
    sleep 1
    ((counter++))
done
log_info "✅ SQL Server is ready!"

# Step 5: Build a custom OpenTelemetry Collector with our receiver
log_info "Building custom OpenTelemetry Collector with New Relic SQL Server receiver..."

# Create builder config
cat > builder-config.yaml << EOF
dist:
  name: otelcol-custom
  description: Custom OpenTelemetry Collector with New Relic SQL Server receiver
  output_path: ./otelcol-custom

exporters:
  - gomod: go.opentelemetry.io/collector/exporter/loggingexporter v0.89.0
  - gomod: go.opentelemetry.io/collector/exporter/otlpexporter v0.89.0
  - gomod: go.opentelemetry.io/collector/exporter/otlphttpexporter v0.89.0

processors:
  - gomod: go.opentelemetry.io/collector/processor/batchprocessor v0.89.0

receivers:
  - gomod: github.com/open-telemetry/opentelemetry-collector-contrib/receiver/newrelicsqlserverreceiver v0.89.0
    path: ./

connectors: []
extensions: []
EOF

# Check if ocb (OpenTelemetry Collector Builder) is installed
if ! command -v ocb &> /dev/null; then
    log_warn "OpenTelemetry Collector Builder (ocb) not found. Installing..."
    go install go.opentelemetry.io/collector/cmd/builder@latest
    if [[ $? -ne 0 ]]; then
        log_error "Failed to install OpenTelemetry Collector Builder"
        exit 1
    fi
fi

# Build the custom collector
log_info "Building custom collector..."
ocb --config builder-config.yaml
if [[ $? -eq 0 ]]; then
    log_info "✅ Custom collector built successfully"
else
    log_error "❌ Failed to build custom collector"
    exit 1
fi

# Step 6: Run the receiver
log_info "Starting the New Relic SQL Server receiver..."
log_info "Check the following:"
log_info "  - Jaeger UI: http://localhost:16686"
log_info "  - Collector logs below"
log_info "  - SQL Server connection: localhost:1433 (sa/YourStrong!Passw0rd)"

echo ""
log_info "Starting collector with your receiver..."
log_warn "Press Ctrl+C to stop"

./otelcol-custom/otelcol-custom --config test-config.yaml

# Cleanup function
cleanup() {
    log_info "Cleaning up..."
    docker-compose -f docker-compose.test.yml down
    log_info "✅ Cleanup complete"
}

# Set trap to cleanup on script exit
trap cleanup EXIT
