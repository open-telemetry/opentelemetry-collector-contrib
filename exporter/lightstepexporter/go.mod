module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lightstepexporter

go 1.14

require (
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/golang/protobuf v1.3.5
	github.com/lightstep/opentelemetry-exporter-go v0.6.2
	github.com/shirou/gopsutil v2.20.4+incompatible // indirect
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.5.1-0.20200721173458-f10fbf228f0e
	go.opentelemetry.io/otel v0.6.0
	go.uber.org/zap v1.15.0
)
