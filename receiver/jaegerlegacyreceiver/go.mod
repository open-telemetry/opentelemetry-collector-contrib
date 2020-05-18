module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerlegacyreceiver

require (
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/golang/protobuf v1.3.5
	github.com/google/go-cmp v0.4.0
	github.com/jaegertracing/jaeger v1.17.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/jaegerthrifthttpexporter v0.0.0-20200403025519-839fe1ccfc10
	github.com/spf13/viper v1.6.2
	github.com/stretchr/testify v1.5.1
	github.com/uber/jaeger-lib v2.2.0+incompatible
	github.com/uber/tchannel-go v1.10.0
	go.opencensus.io v0.22.3
	go.opentelemetry.io/collector v0.3.1-0.20200518164231-3729dac06f74
	go.uber.org/zap v1.10.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/jaegerthrifthttpexporter => ../../exporter/jaegerthrifthttpexporter

go 1.13
