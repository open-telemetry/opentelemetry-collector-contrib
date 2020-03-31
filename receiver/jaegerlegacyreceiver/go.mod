module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerlegacyreceiver

require (
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/golang/protobuf v1.3.5
	github.com/google/go-cmp v0.4.0
	github.com/jaegertracing/jaeger v1.17.0
	github.com/open-telemetry/opentelemetry-collector v0.2.10
	github.com/spf13/viper v1.6.2
	github.com/stretchr/testify v1.4.0
	github.com/uber/jaeger-lib v2.2.0+incompatible
	github.com/uber/tchannel-go v1.10.0
	go.opencensus.io v0.22.3
	go.uber.org/zap v1.10.0
)

go 1.13
