module github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil

go 1.14

require (
	github.com/aws/aws-sdk-go v1.39.5
	github.com/stretchr/testify v1.7.0
	go.uber.org/zap v1.18.1
	golang.org/x/net v0.0.0-20210614182718-04defd469f4e
)

replace go.opentelemetry.io/collector => go.opentelemetry.io/collector v0.29.1-0.20210716020257-4d8e3082465d

replace go.opentelemetry.io/collector/model => go.opentelemetry.io/collector/model v0.0.0-20210716020257-4d8e3082465d
