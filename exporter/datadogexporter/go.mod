module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter

go 1.14

require (
	github.com/DataDog/datadog-agent/pkg/trace/exportable v0.0.0-20201016145401-4646cf596b02
	github.com/aws/aws-sdk-go v1.37.17
	github.com/gogo/protobuf v1.3.2
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.0.0-00010101000000-000000000000
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/stretchr/testify v1.7.0
	github.com/zorkian/go-datadog-api v2.29.0+incompatible // indirect
	go.opentelemetry.io/collector v0.21.0
	go.uber.org/zap v1.16.0
	gopkg.in/DataDog/dd-trace-go.v1 v1.28.0
	gopkg.in/zorkian/go-datadog-api.v2 v2.30.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common
