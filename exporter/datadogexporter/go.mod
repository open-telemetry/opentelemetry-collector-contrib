module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter

go 1.14

replace gopkg.in/zorkian/go-datadog-api.v2 v2.29.0 => github.com/zorkian/go-datadog-api v2.29.1-0.20201007103024-437d51d487bf+incompatible

require (
	github.com/stretchr/testify v1.6.1
	github.com/zorkian/go-datadog-api v2.29.0+incompatible // indirect
	go.opentelemetry.io/collector v0.11.1-0.20201006165100-07236c11fb27
	go.uber.org/zap v1.16.0
	gopkg.in/zorkian/go-datadog-api.v2 v2.29.0
)
