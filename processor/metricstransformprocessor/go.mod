module github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor

go 1.14

require (
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/gogo/protobuf v1.3.1
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.3.0-20200605184202-f640b7103f96
	github.com/stretchr/testify v1.5.1
	go.opentelemetry.io/collector v0.5.0
	go.uber.org/zap v1.13.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common
