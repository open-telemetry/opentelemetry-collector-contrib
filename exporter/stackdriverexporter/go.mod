module github.com/open-telemetry/opentelemetry-service-contrib/exporter/stackdriverexporter

go 1.12

require (
	contrib.go.opencensus.io/exporter/stackdriver v0.12.8-0.20190917133925-4339afab4a99 // TODO: pin a released version
	github.com/open-telemetry/opentelemetry-service v0.0.2-0.20190904165913-41a7afa548c8
	github.com/stretchr/testify v1.3.0
	go.uber.org/zap v1.10.0
	google.golang.org/api v0.10.0
)
