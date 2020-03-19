module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerlegacyreceiver

require (
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/golang/protobuf v1.3.2
	github.com/google/go-cmp v0.3.1
	github.com/jaegertracing/jaeger v1.17.0
	github.com/open-telemetry/opentelemetry-collector v0.2.7
	github.com/spf13/viper v1.4.1-0.20190911140308-99520c81d86e
	github.com/stretchr/testify v1.4.0
	github.com/uber/jaeger-lib v2.2.0+incompatible
	github.com/uber/tchannel-go v1.10.0
	go.opencensus.io v0.22.1
	go.uber.org/zap v1.10.0
)

go 1.13
