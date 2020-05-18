module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombexporter

go 1.14

require (
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/golang/protobuf v1.3.5
	github.com/google/go-cmp v0.4.0
	github.com/honeycombio/opentelemetry-exporter-go v0.3.1
	github.com/klauspost/compress v1.10.2
	github.com/stretchr/testify v1.5.1
	go.opentelemetry.io/collector v0.3.1-0.20200518164231-3729dac06f74
	go.opentelemetry.io/otel v0.3.0
	go.uber.org/zap v1.14.0
)
