module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver

go 1.14

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

require (
	github.com/aws/aws-sdk-go v1.34.9
	github.com/google/uuid v1.1.1
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.8.1-0.20200820203435-961c48b75778
	go.uber.org/zap v1.15.0
)
