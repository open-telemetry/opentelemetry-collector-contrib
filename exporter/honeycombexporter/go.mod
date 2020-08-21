module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombexporter

go 1.14

require (
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/google/go-cmp v0.5.1
	github.com/honeycombio/libhoney-go v1.12.4
	github.com/klauspost/compress v1.10.11
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.8.1-0.20200820203435-961c48b75778
	go.uber.org/zap v1.15.0
	google.golang.org/grpc v1.31.0
	google.golang.org/grpc/examples v0.0.0-20200728194956-1c32b02682df // indirect
	google.golang.org/protobuf v1.25.0
)
