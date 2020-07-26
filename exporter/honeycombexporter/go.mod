module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombexporter

go 1.14

require (
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/golang/protobuf v1.4.2
	github.com/google/go-cmp v0.5.0
	github.com/honeycombio/libhoney-go v1.12.4
	github.com/klauspost/compress v1.10.10
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.5.1-0.20200723232356-d4053cc823a0
	go.uber.org/zap v1.15.0
	google.golang.org/grpc v1.29.1
)
