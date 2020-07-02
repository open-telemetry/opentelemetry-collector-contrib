module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cudareceiver

go 1.14

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

require (
	github.com/census-instrumentation/opencensus-proto v0.2.1
	go.opentelemetry.io/collector v0.5.0
	go.uber.org/zap v1.13.0
)
