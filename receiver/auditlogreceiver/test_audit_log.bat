@echo off
echo Testing Audit Log Receiver...

echo Test 1: Sending simple audit log...
curl -X POST http://localhost:8080/v1/logs -H "Content-Type: application/json" -d "{\"event\": \"user_login\", \"user\": \"john.doe\", \"timestamp\": \"2024-01-01T00:00:00Z\", \"ip\": \"192.168.1.100\"}"

echo.
echo Test 2: Sending audit log with OTLP protobuf content type...
curl -X POST http://localhost:8080/v1/logs -H "Content-Type: application/x-protobuf" -d "{\"event\": \"user_logout\", \"user\": \"jane.smith\", \"timestamp\": \"2024-01-01T00:05:00Z\", \"ip\": \"192.168.1.101\"}"

echo.
echo Test 3: Sending multiple audit logs...
for /L %%i in (1,1,5) do (
    curl -X POST http://localhost:8080/v1/logs -H "Content-Type: application/json" -d "{\"event\": \"api_call\", \"user\": \"user%%i\", \"timestamp\": \"2024-01-01T00:0%%i:00Z\", \"endpoint\": \"/api/v1/data\"}"
    echo.
)

echo All tests completed!
pause
