module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombexporter

go 1.14

require (
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/golang/protobuf v1.3.5
	github.com/google/go-cmp v0.4.0
	github.com/honeycombio/libhoney-go v1.12.3
	github.com/klauspost/compress v1.10.2
	github.com/opentracing/opentracing-go v1.1.1-0.20190913142402-a7454ce5950e // indirect
	github.com/stretchr/testify v1.5.1
	go.opentelemetry.io/collector v0.4.1-0.20200629224201-e7a7690e21fc
	go.uber.org/zap v1.14.0
	google.golang.org/grpc v1.29.1
)
