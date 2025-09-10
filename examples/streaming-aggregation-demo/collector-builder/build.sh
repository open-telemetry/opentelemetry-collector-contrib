#!/bin/bash

# Build script for custom OpenTelemetry Collector with Streaming Aggregation Processor

set -e

echo "Building custom OpenTelemetry Collector with Streaming Aggregation Processor..."

# Create bin directory if it doesn't exist
mkdir -p bin

# Build the collector using go build
echo "Building collector binary..."
cd "$(dirname "$0")"

# Download dependencies
echo "Downloading dependencies..."
go mod download

# Build the binary
echo "Compiling..."
go build -o bin/otelcol-streaming main.go

echo "Build complete! Binary available at ./bin/otelcol-streaming"

# Make the binary executable
chmod +x ./bin/otelcol-streaming

echo "Done!"
