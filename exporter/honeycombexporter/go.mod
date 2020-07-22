module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombexporter

go 1.14

require (
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/golang/protobuf v1.3.5
	github.com/google/go-cmp v0.5.0
	github.com/honeycombio/libhoney-go v1.12.4
	github.com/klauspost/compress v1.10.10
	github.com/stretchr/testify v1.6.1
	go.opencensus.io v0.22.4 // indirect
	go.opentelemetry.io/collector v0.5.1-0.20200721173458-f10fbf228f0e
	go.uber.org/zap v1.15.0
	google.golang.org/grpc v1.29.1
)
