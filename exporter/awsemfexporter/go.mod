module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter

go 1.14

require (
	github.com/aws/aws-sdk-go v1.35.20
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/golang/protobuf v1.4.2
	github.com/google/uuid v1.1.2
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.13.1-0.20201101004512-f4e4382d0e0e
	go.uber.org/zap v1.16.0
)
