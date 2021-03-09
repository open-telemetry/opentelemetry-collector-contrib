module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter

go 1.14

require (
	github.com/aws/aws-sdk-go v1.37.26
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/golang/protobuf v1.4.3
	github.com/google/uuid v1.2.0
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.21.1-0.20210308033310-65c4c4a1b383
	go.opentelemetry.io/otel v0.18.0
	go.uber.org/zap v1.16.0
	google.golang.org/protobuf v1.25.0
)
