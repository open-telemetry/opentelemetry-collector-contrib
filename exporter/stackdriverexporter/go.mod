module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stackdriverexporter

go 1.12

require (
	code.cloudfoundry.org/clock v0.0.0-20180518195852-02e53af36e6c // indirect
	// TODO: pin a released version
	contrib.go.opencensus.io/exporter/stackdriver v0.12.8-0.20190917133925-4339afab4a99
	github.com/Microsoft/ApplicationInsights-Go v0.4.2 // indirect
	github.com/open-telemetry/opentelemetry-collector v0.2.1-0.20191016224815-dfabfb0c1d1e
	github.com/stretchr/testify v1.4.0
	go.uber.org/zap v1.10.0
	google.golang.org/api v0.10.0
	google.golang.org/grpc v1.23.1
)
