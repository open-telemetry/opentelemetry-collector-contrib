module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter

go 1.16

require (
	github.com/DataDog/datadog-agent/pkg/trace/exportable v0.0.0-20201016145401-4646cf596b02
	github.com/OneOfOne/xxhash v1.2.5 // indirect
	github.com/armon/go-metrics v0.3.3 // indirect
	github.com/aws/aws-sdk-go v1.38.64
	github.com/gogo/protobuf v1.3.2
	github.com/hashicorp/go-immutable-radix v1.2.0 // indirect
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.0.0-00010101000000-000000000000
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pelletier/go-toml v1.8.0 // indirect
	github.com/stretchr/testify v1.7.0
	github.com/tinylib/msgp v1.1.5 // indirect
	github.com/zorkian/go-datadog-api v2.29.0+incompatible // indirect
	go.opentelemetry.io/collector v0.28.1-0.20210616151306-cdc163427b8e
	go.uber.org/zap v1.17.0
	gopkg.in/DataDog/dd-trace-go.v1 v1.31.1
	gopkg.in/ini.v1 v1.57.0 // indirect
	gopkg.in/zorkian/go-datadog-api.v2 v2.30.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common
