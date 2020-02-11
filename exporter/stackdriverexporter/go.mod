module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stackdriverexporter

go 1.12

require (
	// TODO: pin a released version
	contrib.go.opencensus.io/exporter/stackdriver v0.12.8-0.20190917133925-4339afab4a99
	github.com/open-telemetry/opentelemetry-collector v0.2.6
	github.com/stretchr/testify v1.4.0
	go.uber.org/zap v1.10.0
	google.golang.org/api v0.10.0
	google.golang.org/grpc v1.23.1
)
