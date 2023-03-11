module github.com/open-telemetry/opentelemetry-collector-contrib

go 1.19

// required for tests
require (
	github.com/google/uuid v1.3.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector v0.35.2 // indirect
	go.opentelemetry.io/collector v0.73.0 //indirect
	go.opentelemetry.io/collector/exporter v0.73.0 //indirect
	go.opentelemetry.io/collector/receiver v0.73.0 //indirect
)
