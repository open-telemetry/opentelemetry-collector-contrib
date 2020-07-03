module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cudareceiver

go 1.14

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

require (
	contrib.go.opencensus.io/resource v0.1.2
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/golang/protobuf v1.3.5
	go.opentelemetry.io/collector v0.5.0
	go.uber.org/zap v1.13.0
)
