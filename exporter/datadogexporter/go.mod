module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter

go 1.16

require (
	github.com/DataDog/datadog-agent/pkg/trace/exportable v0.0.0-20201016145401-4646cf596b02
	github.com/aws/aws-sdk-go v1.40.19
	github.com/gogo/protobuf v1.3.2
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.0.0-00010101000000-000000000000
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/stretchr/testify v1.7.0
	github.com/tinylib/msgp v1.1.5 // indirect
	github.com/zorkian/go-datadog-api v2.29.0+incompatible // indirect
	go.opentelemetry.io/collector v0.31.1-0.20210810171211-8038673eba9e
	go.opentelemetry.io/collector/model v0.31.1-0.20210810171211-8038673eba9e
	go.uber.org/zap v1.19.0
	gopkg.in/DataDog/dd-trace-go.v1 v1.31.1
	gopkg.in/zorkian/go-datadog-api.v2 v2.30.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common
