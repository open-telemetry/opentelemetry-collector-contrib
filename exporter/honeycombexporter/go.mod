module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombexporter

go 1.14

require (
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/golang/protobuf v1.3.5
	github.com/google/go-cmp v0.4.0
	github.com/honeycombio/opentelemetry-exporter-go v0.4.1-0.20200410161359-66047b9d25e7
	github.com/klauspost/compress v1.10.3
	github.com/stretchr/testify v1.5.1
	go.opentelemetry.io/collector v0.3.1-0.20200515190405-a637b41c22e3
	go.opentelemetry.io/otel v0.4.2
	go.uber.org/zap v1.14.0
	golang.org/x/net v0.0.0-20200505041828-1ed23360d12c // indirect
	google.golang.org/appengine v1.6.6 // indirect
	google.golang.org/grpc v1.29.1 // indirect
)
