module github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray

go 1.16

require (
	github.com/aws/aws-sdk-go v1.39.5
	github.com/stretchr/testify v1.7.0
)

replace go.opentelemetry.io/collector => go.opentelemetry.io/collector v0.29.1-0.20210716020257-4d8e3082465d

replace go.opentelemetry.io/collector/model => go.opentelemetry.io/collector/model v0.0.0-20210716020257-4d8e3082465d
