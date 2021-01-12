module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter

go 1.14

require (
	github.com/aws/aws-sdk-go v1.36.24
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/golang/protobuf v1.4.3
	github.com/google/uuid v1.1.4
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.17.1-0.20210106191745-ca6f1a0287d0
	go.uber.org/zap v1.16.0
)
