module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinscribereceiver

go 1.12

require (
	github.com/apache/thrift v0.0.0-20161221203622-b2a4d4ae21c7
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/golang/protobuf v1.3.2
	github.com/jaegertracing/jaeger v1.17.0
	github.com/omnition/scribe-go v1.0.0
	github.com/open-telemetry/opentelemetry-collector v0.2.6
	github.com/stretchr/testify v1.4.0
	go.uber.org/zap v1.10.0
)

replace github.com/open-telemetry/opentelemetry-collector => ../../../opentelemetry-collector
