# Makefile for New Relic SQL Server receiver testing

.PHONY: test test-unit test-integration test-docker clean setup-test-env

# Default target
test: test-unit

# Run unit tests
test-unit:
	@echo "Running unit tests..."
	cd .. && go test -v .

# Run unit tests with coverage
test-coverage:
	@echo "Running unit tests with coverage..."
	cd .. && go test -v -race -coverprofile=testdata/coverage.out .
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: testdata/coverage.html"

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	./run-integration-test.sh

# Run integration tests (Go)
test-integration-go:
	@echo "Running Go integration tests..."
	go test -tags=integration -v .

# Build custom collector for testing
build-collector:
	@echo "Building custom collector..."
	@if command -v ocb >/dev/null 2>&1; then \
		ocb --config builder-config.yaml; \
	else \
		echo "OpenTelemetry Collector Builder (ocb) not found. Please install it:"; \
		echo "  go install go.opentelemetry.io/collector/cmd/builder@latest"; \
	fi

# Start test environment
start-test-env:
	@echo "Starting test environment..."
	cd testdata && docker-compose up -d
	@echo "Waiting for services to be ready..."
	@sleep 30
	@echo "Test environment is ready!"
	@echo "SQL Server: localhost:1433 (sa/TestPassword123!)"
	@echo "Jaeger UI: http://localhost:16686"
	@echo "Collector metrics: http://localhost:8888/metrics"

# Stop test environment
stop-test-env:
	@echo "Stopping test environment..."
	cd testdata && docker-compose down -v

# Setup test database with sample data
setup-test-data:
	@echo "Setting up test data..."
	@cd testdata && \
	docker-compose exec -T sqlserver /opt/mssql-tools/bin/sqlcmd -S localhost -U sa -P TestPassword123! -i "/docker-entrypoint-initdb.d/01-init-database.sql" && \
	docker-compose exec -T sqlserver /opt/mssql-tools/bin/sqlcmd -S localhost -U sa -P TestPassword123! -i "/docker-entrypoint-initdb.d/02-create-test-procedures.sql"
	@echo "Test data setup complete!"

# Generate test activity in the database
generate-activity:
	@echo "Generating test activity..."
	cd testdata && docker-compose exec -T sqlserver /opt/mssql-tools/bin/sqlcmd -S localhost -U sa -P TestPassword123! -d TestDatabase -Q "EXEC GenerateTestActivity"

# View collector logs
logs:
	cd testdata && docker-compose logs -f otel-collector

# View SQL Server logs
logs-sql:
	cd testdata && docker-compose logs -f sqlserver

# Check receiver status
status:
	@echo "Checking receiver status..."
	@curl -s http://localhost:8888/metrics | grep newrelic || echo "Receiver metrics not found"

# Validate configuration
validate-config:
	@echo "Validating collector configuration..."
	@if [ -f testdata/bin/otelcol-dev ]; then \
		cd testdata && ./bin/otelcol-dev --config-file=config.yaml --dry-run; \
	else \
		echo "Custom collector not found. Run 'make build-collector' first."; \
	fi

# Clean up generated files
clean:
	@echo "Cleaning up..."
	rm -f coverage.out coverage.html
	rm -rf testdata/bin/
	cd testdata && docker-compose down -v
	docker system prune -f

# Install test dependencies
install-deps:
	@echo "Installing test dependencies..."
	go mod tidy
	@if ! command -v ocb >/dev/null 2>&1; then \
		echo "Installing OpenTelemetry Collector Builder..."; \
		go install go.opentelemetry.io/collector/cmd/builder@latest; \
	fi

# Quick test - start environment and run basic tests
quick-test: start-test-env setup-test-data
	@echo "Running quick integration test..."
	@sleep 10
	@make generate-activity
	@sleep 5
	@make status
	@make stop-test-env

help:
	@echo "Available targets:"
	@echo "  test              - Run unit tests"
	@echo "  test-coverage     - Run unit tests with coverage"
	@echo "  test-integration  - Run full integration tests"
	@echo "  build-collector   - Build custom collector with receiver"
	@echo "  start-test-env    - Start Docker test environment"
	@echo "  stop-test-env     - Stop Docker test environment"
	@echo "  setup-test-data   - Initialize test database with sample data"
	@echo "  generate-activity - Generate database activity for testing"
	@echo "  logs              - View collector logs"
	@echo "  logs-sql          - View SQL Server logs"
	@echo "  status            - Check receiver status"
	@echo "  validate-config   - Validate collector configuration"
	@echo "  clean             - Clean up generated files and containers"
	@echo "  install-deps      - Install test dependencies"
	@echo "  quick-test        - Run a quick end-to-end test"
	@echo "  help              - Show this help message"
