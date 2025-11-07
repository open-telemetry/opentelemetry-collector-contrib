#!/bin/bash

# Test script to verify Hydrolix connectivity and data ingestion

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "Testing Hydrolix Connection..."
echo "================================"
echo ""

# Check if credentials are set
if [ -z "$HYDROLIX_USERNAME" ] || [ -z "$HYDROLIX_PASSWORD" ]; then
    echo -e "${RED}ERROR: HYDROLIX_USERNAME and HYDROLIX_PASSWORD must be set${NC}"
    echo "Export them in your shell:"
    echo "  export HYDROLIX_USERNAME=your_username"
    echo "  export HYDROLIX_PASSWORD=your_password"
    exit 1
fi

ENDPOINT="https://argus.hydrolix.live/ingest/event"
TABLE="cicd.metricsV2"
TRANSFORM="tf_cicd_metrics_v2"

echo -e "${YELLOW}Testing Metrics Endpoint...${NC}"
echo "Endpoint: $ENDPOINT"
echo "Table: $TABLE"
echo "Transform: $TRANSFORM"
echo ""

# Create a minimal test payload
TEST_PAYLOAD='[
  {
    "name": "test_metric",
    "metric_type": "gauge",
    "timestamp": '$(date +%s)000000000',
    "value": 42.0,
    "tags": [{"test": "true"}],
    "serviceTags": [{"service.name": "test-service"}],
    "serviceName": "test-service"
  }
]'

echo "Sending test metric..."
echo ""

RESPONSE=$(curl -w "\nHTTP_CODE:%{http_code}" -X POST "$ENDPOINT" \
  -H "Content-Type: application/json" \
  -H "x-hdx-table: $TABLE" \
  -H "x-hdx-transform: $TRANSFORM" \
  -u "$HYDROLIX_USERNAME:$HYDROLIX_PASSWORD" \
  -d "$TEST_PAYLOAD" \
  2>&1)

HTTP_CODE=$(echo "$RESPONSE" | grep "HTTP_CODE:" | cut -d: -f2)
BODY=$(echo "$RESPONSE" | sed '/HTTP_CODE:/d')

echo "Response:"
echo "$BODY"
echo ""
echo "HTTP Status Code: $HTTP_CODE"
echo ""

if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "204" ]; then
    echo -e "${GREEN}✓ SUCCESS: Metrics endpoint is working!${NC}"
    echo ""
    echo "If metrics still aren't appearing:"
    echo "1. Check your Hydrolix transform configuration"
    echo "2. Verify the table schema matches the payload structure"
    echo "3. Check Hydrolix query results with a slight delay (processing time)"
else
    echo -e "${RED}✗ FAILED: HTTP $HTTP_CODE${NC}"
    echo ""
    echo "Common issues:"
    echo "- 401/403: Check your HYDROLIX_USERNAME and HYDROLIX_PASSWORD"
    echo "- 404: Verify the endpoint URL, table name, or transform name"
    echo "- 500: Check Hydrolix transform or table configuration"
fi
