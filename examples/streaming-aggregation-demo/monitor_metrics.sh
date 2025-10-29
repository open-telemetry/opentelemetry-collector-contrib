#!/bin/bash

echo "Monitoring aggregated metrics to verify 30-second window updates..."
echo "Timestamp | requests_per_second | active_connections | response_time_p50"
echo "----------------------------------------------------------------------"

for i in {1..70}; do
    timestamp=$(date +"%H:%M:%S")
    
    # Query aggregated metrics
    requests=$(curl -s http://localhost:9090/api/v1/query?query=aggregated_requests_per_second 2>/dev/null | jq -r '.data.result[0].value[1] // "N/A"' 2>/dev/null || echo "N/A")
    connections=$(curl -s http://localhost:9090/api/v1/query?query=aggregated_active_connections 2>/dev/null | jq -r '.data.result[0].value[1] // "N/A"' 2>/dev/null || echo "N/A")
    p50=$(curl -s http://localhost:9090/api/v1/query?query='aggregated_response_time_bucket{quantile="0.5"}' 2>/dev/null | jq -r '.data.result[0].value[1] // "N/A"' 2>/dev/null || echo "N/A")
    
    echo "$timestamp | $requests | $connections | $p50"
    
    sleep 1
done

echo ""
echo "If the fix is working correctly, the aggregated values should only change every ~30 seconds."
