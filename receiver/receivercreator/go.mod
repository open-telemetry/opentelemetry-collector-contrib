module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receiver_creator

go 1.14

require (
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/open-telemetry/opentelemetry-collector v0.3.1-0.20200503151053-5d1aacc0e168
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer v0.0.0
	github.com/spf13/cast v1.3.1
	github.com/spf13/viper v1.6.2
	github.com/stretchr/testify v1.5.1
	go.uber.org/zap v1.13.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer => ../../extension/observer
