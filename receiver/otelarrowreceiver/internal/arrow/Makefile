# Generate the mock files
.PHONY: mockgen
mockgen:
	go install go.uber.org/mock/mockgen@latest
	mkdir -p ./mock
	mockgen -package mock go.opentelemetry.io/collector/extension/auth Server > mock/auth.go
	mockgen -package mock go.opentelemetry.io/collector/consumer Traces,Metrics,Logs > mock/consumer.go
