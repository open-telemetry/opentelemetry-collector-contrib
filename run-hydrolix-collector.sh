#!/bin/bash

# Set your Hydrolix credentials as environment variables do
# Do not add them here!
#export HYDROLIX_USERNAME="your_username_here"
#export HYDROLIX_PASSWORD="your_password_here"

# Find the built collector binary
COLLECTOR_BIN=$(find ./bin -name "otelcontribcol*" -type f | head -1)

if [ -z "$COLLECTOR_BIN" ]; then
    echo "Error: Collector binary not found in ./bin/"
    echo "Please run 'make otelcontribcol' first"
    exit 1
fi

echo "Using collector binary: $COLLECTOR_BIN"
echo "Starting collector with Hydrolix exporter..."

# Run the collector with the config
$COLLECTOR_BIN --config=hydrolix-config.yaml