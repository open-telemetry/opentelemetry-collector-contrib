# Testing Your New Relic SQL Server Receiver

This guide shows you how to test your receiver with a real SQL Server instance and OTLP endpoint.

## Quick Start (5 minutes)

```bash
# 1. Start the test environment
cd testdata
docker-compose up -d

# 2. Wait for SQL Server to be ready (about 30 seconds)
docker-compose logs -f sqlserver  # Watch for "SQL Server is now ready"

# 3. Set up test data
make setup-test-data

# 4. View metrics being collected
make logs

# 5. Generate some database activity
make generate-activity

# 6. Check metrics in Jaeger UI
open http://localhost:16686
```

## Test with New Relic OTLP Endpoint

```bash
# 1. Set your New Relic API key
export NEW_RELIC_API_KEY="your-license-key-here"

# 2. Update the config to use New Relic endpoint
# Edit testdata/integration/config-full.yaml
# Uncomment the line: exporters: [debug, otlp/newrelic, otlp/local]

# 3. Run with the full config
cd testdata
docker-compose down
docker run --rm \
  -v $(pwd)/integration:/etc/config \
  -e NEW_RELIC_API_KEY=$NEW_RELIC_API_KEY \
  --network host \
  otel/opentelemetry-collector-contrib:latest \
  --config=/etc/config/config-full.yaml

# 4. Check New Relic dashboard for metrics
```

## Available Test Configurations

| Config File | Purpose | OTLP Endpoint |
|-------------|---------|---------------|
| `config.yaml` | Local testing | Jaeger (localhost:14250) |
| `config-minimal.yaml` | Basic setup | Debug output only |
| `config-azure.yaml` | Azure SQL Database | New Relic |
| `integration/config-full.yaml` | Full feature test | Both New Relic & Jaeger |

## Test Scenarios

### 1. Basic Metrics Collection
```bash
# Collects: User connections, buffer metrics, disk metrics
make start-test-env
make setup-test-data
# Check logs for: sqlserver.stats.connections
```

### 2. Query Performance Monitoring
```bash
# Tests blocking session detection
make start-test-env
make setup-test-data

# Create blocking sessions in background
docker-compose exec -d sqlserver /opt/mssql-tools/bin/sqlcmd \
  -S localhost -U sa -P TestPassword123! \
  -d TestDatabase -Q "EXEC CreateBlockingSession"

# Check for blocking metrics in logs
make logs | grep -i blocking
```

### 3. Load Testing
```bash
# Generate continuous activity
make start-test-env
make setup-test-data

# Run multiple activities simultaneously
for i in {1..5}; do
  make generate-activity &
done

# Monitor resource usage
make status
```

## Verify Metrics

### Expected Metrics
- `sqlserver.stats.connections` - Current user connections
- More metrics will be added as you implement them

### Check via HTTP endpoint
```bash
curl http://localhost:8888/metrics | grep sqlserver
```

### Check in Jaeger UI
1. Open http://localhost:16686
2. Look for service: `newrelic-sqlserver-receiver-test`
3. View traces to see metric collection

## Troubleshooting

### Receiver not starting
```bash
# Check configuration
make validate-config

# Check SQL Server connectivity
docker-compose exec otel-collector ping sqlserver
```

### No metrics appearing
```bash
# Check collector logs
make logs

# Verify SQL Server has data
docker-compose exec sqlserver /opt/mssql-tools/bin/sqlcmd \
  -S localhost -U sa -P TestPassword123! \
  -Q "SELECT cntr_value FROM sys.dm_os_performance_counters WHERE counter_name = 'User Connections'"
```

### Connection errors
```bash
# Check SQL Server status
docker-compose exec sqlserver /opt/mssql-tools/bin/sqlcmd \
  -S localhost -U sa -P TestPassword123! -Q "SELECT @@VERSION"

# Check network
docker-compose exec otel-collector nslookup sqlserver
```

## Integration with New Relic

### Set up New Relic account
1. Sign up at https://newrelic.com
2. Get your License Key from Account Settings
3. Set `NEW_RELIC_API_KEY` environment variable

### View metrics in New Relic
1. Go to New Relic Dashboards
2. Create custom dashboard
3. Add charts for your SQL Server metrics
4. Use NRQL queries like: `SELECT * FROM Metric WHERE metricName LIKE 'sqlserver.%'`

## Running Tests

```bash
# Unit tests
make test

# Unit tests with coverage
make test-coverage

# Integration tests (requires Docker)
make test-integration

# Manual integration test
go test -tags=integration -v ./...
```

## Cleanup

```bash
# Stop and remove all containers
make clean

# Or manually
cd testdata
docker-compose down -v
docker system prune -f
```

## Next Steps

1. **Add More Metrics**: Implement additional SQL Server performance counters
2. **Custom Queries**: Add support for custom SQL queries
3. **Alerting**: Set up alerts in New Relic based on your metrics
4. **Dashboard**: Create comprehensive SQL Server monitoring dashboard
5. **Production**: Deploy to your production environment

This testing setup gives you a complete environment to validate your receiver works correctly with real SQL Server instances and can send metrics to OTLP endpoints like New Relic.
