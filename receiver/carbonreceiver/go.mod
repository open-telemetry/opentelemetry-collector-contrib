module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver

go 1.13

require (
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/golang/protobuf v1.3.2
	github.com/open-telemetry/opentelemetry-collector v0.2.6
	github.com/spf13/viper v1.6.2
	github.com/stretchr/testify v1.4.0
	go.opencensus.io v0.22.1
	go.uber.org/zap v1.13.0
)

replace github.com/open-telemetry/opentelemetry-collector => ../../../opentelemetry-collector
