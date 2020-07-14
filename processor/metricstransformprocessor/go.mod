module github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor

go 1.14

require (
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/gogo/protobuf v1.3.1
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.5.1-0.20200713215410-08e53d465374
	go.uber.org/zap v1.13.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common
