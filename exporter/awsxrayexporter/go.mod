module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter

go 1.16

require (
	github.com/armon/go-metrics v0.3.3 // indirect
	github.com/aws/aws-sdk-go v1.38.64
	github.com/hashicorp/go-immutable-radix v1.2.0 // indirect
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray v0.0.0-00010101000000-000000000000
	github.com/pelletier/go-toml v1.8.0 // indirect
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.28.1-0.20210616151306-cdc163427b8e
	go.uber.org/zap v1.17.0
	gopkg.in/ini.v1 v1.57.0 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray => ./../../internal/aws/xray

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil => ./../../internal/aws/awsutil
