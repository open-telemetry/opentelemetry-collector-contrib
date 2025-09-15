# New Relic SQL Server Receiver Testing

This directory contains test infrastructure for the New Relic SQL Server receiver, including integration tests, Docker setup, and sample configurations.

## Quick Start

### Prerequisites

- Docker and Docker Compose
- Go 1.19+ (for building)
- Make (optional, for convenience commands)

### Run Integration Tests

```bash
# Option 1: Automated integration tests (recommended)
cd ..  # Go to receiver root directory
go test -tags integration -v ./...

# Option 2: Manual testing with Docker
./run-integration-test.sh

# Option 3: Using Make
make test-integration

# Option 4: Manual steps
make start-test-env
make setup-test-data
make logs  # View collector output
make stop-test-env
```

The automated integration tests (Option 1) use testcontainers to automatically:
- Start SQL Server 2022 containers
- Initialize databases and users  
- Test receiver functionality
- Clean up containers when done

## Test Environment Components

### SQL Server
- **Container**: `mcr.microsoft.com/mssql/server:2022-latest`
- **Port**: `1433`
- **Credentials**: `sa / TestPassword123!`
- **Test Database**: `TestDatabase`

### OpenTelemetry Collector
- **Port**: `4317` (OTLP gRPC), `4318` (OTLP HTTP)
- **Metrics**: `8888` (Prometheus format)
- **Config**: Based on `config.yaml`

### Jaeger (OTLP Endpoint)
- **UI**: http://localhost:16686
- **OTLP**: `14250`

## Test Configurations

### Basic Testing (`config.yaml`)
Standard configuration for local testing with debug output.

### Minimal Testing (`config-minimal.yaml`)
Minimal configuration with basic settings.

### Azure SQL (`config-azure.yaml`)
Configuration for testing with Azure SQL Database using Azure AD authentication.

### Full Integration (`integration/config-full.yaml`)
Complete configuration with New Relic OTLP endpoint, processors, and multiple exporters.

## Test Data

The test environment automatically creates:

### Database Objects
- `TestDatabase` database
- `Orders` and `Customers` tables with sample data
- Indexes for performance testing
- `CustomerOrderSummary` view

### Test Procedures
- `GenerateTestActivity` - Creates random orders
- `CreateBlockingSession` - Simulates blocking for query monitoring
- `SimulateLongRunningQuery` - Generates complex queries
- `GenerateOngoingActivity` - Creates continuous database activity

## Testing Scenarios

### 1. Basic Metrics Collection
```bash
make start-test-env
make setup-test-data
# Wait 30 seconds, then check logs
make logs
```

### 2. Query Performance Monitoring
```bash
make start-test-env
make setup-test-data

# Generate blocking sessions
docker-compose exec sqlserver /opt/mssql-tools/bin/sqlcmd \
  -S localhost -U sa -P TestPassword123! \
  -d TestDatabase -Q "EXEC CreateBlockingSession" &

# Check for blocking session metrics
make logs | grep -i blocking
```

### 3. Load Testing
```bash
make start-test-env
make setup-test-data

# Generate continuous activity
for i in {1..10}; do
  make generate-activity &
done

# Monitor metrics
make status
```

### 4. New Relic OTLP Testing
```bash
# Set your New Relic API key
export NEW_RELIC_API_KEY="your-api-key-here"

# Use the full integration config
docker-compose -f docker-compose.yml up -d sqlserver
docker run --rm -v $(pwd)/integration:/config \
  -e NEW_RELIC_API_KEY=$NEW_RELIC_API_KEY \
  --network newrelicsqlserverreceiver_sqlserver-test \
  otel/opentelemetry-collector-contrib:latest \
  --config=/config/config-full.yaml
```

## Configuration Examples

### Environment Variables
```bash
export SQLSERVER_PASSWORD="TestPassword123!"
export NEW_RELIC_API_KEY="your-api-key"
export AZURE_CLIENT_ID="your-client-id"
export AZURE_TENANT_ID="your-tenant-id"
export AZURE_CLIENT_SECRET="your-client-secret"
```

### Custom Receiver Configuration
```yaml
receivers:
  newrelicsqlserver:
    hostname: localhost
    port: "1433"
    username: sa
    password: ${SQLSERVER_PASSWORD}
    collection_interval: 15s
    enable_query_monitoring: true
    query_monitoring_response_time_threshold: 2
```

## Troubleshooting

### SQL Server Connection Issues
```bash
# Check SQL Server status
docker-compose exec sqlserver /opt/mssql-tools/bin/sqlcmd \
  -S localhost -U sa -P TestPassword123! -Q "SELECT @@VERSION"

# Check network connectivity
docker-compose exec otel-collector ping sqlserver
```

### Collector Issues
```bash
# Validate configuration
make validate-config

# Check collector logs
make logs

# Check metrics endpoint
curl http://localhost:8888/metrics
```

### Missing Metrics
```bash
# Check if receiver is collecting data
make logs | grep -i "newrelicsqlserver"

# Generate database activity
make generate-activity

# Check SQL Server performance counters
docker-compose exec sqlserver /opt/mssql-tools/bin/sqlcmd \
  -S localhost -U sa -P TestPassword123! \
  -Q "SELECT * FROM sys.dm_os_performance_counters WHERE counter_name = 'User Connections'"
```

## Make Commands

| Command | Description |
|---------|-------------|
| `make test` | Run unit tests |
| `make test-integration` | Run full integration tests |
| `make start-test-env` | Start Docker test environment |
| `make stop-test-env` | Stop and cleanup test environment |
| `make setup-test-data` | Initialize test database |
| `make generate-activity` | Create database activity |
| `make logs` | View collector logs |
| `make status` | Check receiver metrics |
| `make clean` | Clean up all test artifacts |

## Files Structure

```
testdata/
├── Makefile                      # Test automation
├── README.md                     # This file
├── run-integration-test.sh       # Integration test script
├── docker-compose.yml            # Test environment
├── config.yaml                   # Basic collector config
├── config-minimal.yaml          # Minimal config
├── config-azure.yaml            # Azure SQL config
├── sql-scripts/                  # Database setup scripts
│   ├── 01-init-database.sql
│   └── 02-create-test-procedures.sql
├── scraper/                      # Expected metrics
│   └── expected.yaml
└── integration/                  # Integration test configs
    └── config-full.yaml
```

## Continuous Testing

For automated testing in CI/CD:

```bash
# Run all tests
make test-unit test-integration

# Run with coverage
make test-coverage

# Cleanup
make clean
```
