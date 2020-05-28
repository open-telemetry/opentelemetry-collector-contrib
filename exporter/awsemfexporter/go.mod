module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter

go 1.14

require (
	github.com/aws/aws-sdk-go v1.23.19
	github.com/golang/protobuf v1.3.5
	github.com/mattn/go-isatty v0.0.10 // indirect
	github.com/stretchr/testify v1.5.1
	go.opentelemetry.io/collector v0.3.1-0.20200522130256-daf2fc71ac65
	go.uber.org/zap v1.10.0
	golang.org/x/net v0.0.0-20200324143707-d3edc9973b7e
)