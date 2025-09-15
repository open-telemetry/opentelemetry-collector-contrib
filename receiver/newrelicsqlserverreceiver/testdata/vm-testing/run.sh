#!/bin/bash

# Alternative approach: Run the collector directly from the contrib codebase
# This avoids OCB version conflicts and uses your local receiver changes directly

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONTRIB_ROOT="/Users/tanushangural/IdeaProjects/Forks/nri-mssql-and-otel-contrib/opentelemetry-collector-contrib"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}🚀 Running OpenTelemetry Collector with New Relic SQL Server Receiver${NC}"
echo ""

# Check if config exists
if [[ ! -f "${SCRIPT_DIR}/config.yaml" ]]; then
    echo -e "${RED}❌ config.yaml not found in ${SCRIPT_DIR}${NC}"
    echo "Make sure you have the config file with your database credentials and New Relic API key"
    exit 1
fi

# Since credentials are in config.yaml, we don't need environment variables
echo -e "${GREEN}✅ Using credentials from config.yaml file${NC}"
echo -e "${YELLOW}Server:${NC} 20.235.136.68:1433"
echo -e "${YELLOW}Username:${NC} newrelic"
echo -e "${YELLOW}Database:${NC} master"
echo -e "${YELLOW}New Relic Endpoint:${NC} staging-otlp.nr-data.net"
echo ""

# Go to contrib root and build the collector with your receiver
echo -e "${YELLOW}Building and running collector with your receiver changes...${NC}"
cd "${CONTRIB_ROOT}"

# Run the collector directly with go run, using your config
echo -e "${GREEN}🎯 Starting OpenTelemetry Collector...${NC}"
echo -e "${YELLOW}Press Ctrl+C to stop${NC}"
echo ""

# Go to the otelcontribcol directory and run from there
cd ./cmd/otelcontribcol && go run . --config="${SCRIPT_DIR}/config.yaml"
