module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lightstepexporter

go 1.14

replace go.opentelemetry.io/otel => ../../../../../go.opentelemetry.io
replace go.opentelemetry.io/otel/exporters/otlp => ../../../../../go.opentelemetry.io/exporters/otlp

require (
	github.com/lightstep/opentelemetry-exporter-go v0.5.0
	github.com/stretchr/testify v1.5.1
	go.opentelemetry.io/collector v0.3.1-0.20200518164231-3729dac06f74
	go.opentelemetry.io/otel v0.5.0
	go.opentelemetry.io/otel/exporters/otlp v0.5.0
	go.uber.org/zap v1.14.0
)

replace github.com/apache/thrift => github.com/apache/thrift v0.0.0-20161221203622-b2a4d4ae21c7
