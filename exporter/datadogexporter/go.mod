module github.com/DataDog/opentelemetry-collector-contrib/exporter/datadogexporter

go 1.15

require (
	github.com/DataDog/datadog-agent v0.0.0-20200417180928-f454c60bc16f // indirect
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/stretchr/testify v1.6.1
	github.com/zorkian/go-datadog-api v2.29.0+incompatible // indirect
	go.opencensus.io v0.22.4
	go.opentelemetry.io/collector v0.10.1-0.20200917170114-639b9a80ed46
	go.uber.org/zap v1.16.0
	gopkg.in/DataDog/dd-trace-go.v1 v1.26.0
	gopkg.in/zorkian/go-datadog-api.v2 v2.29.0
)
