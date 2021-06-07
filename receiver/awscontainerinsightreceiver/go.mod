module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver

go 1.16

require (
	github.com/aws/aws-sdk-go v1.38.51
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil v0.0.0-00010101000000-000000000000
	github.com/shirou/gopsutil v3.21.4+incompatible
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.27.1-0.20210603182316-5369d7e9e83e
	go.uber.org/zap v1.17.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil => ./../../internal/aws/awsutil
