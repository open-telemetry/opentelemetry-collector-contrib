module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/influxbexporter

go 1.15

require (
	github.com/influxdata/influxdb-observability/common v0.0.0-20210429161704-67280d9a921b
	github.com/influxdata/influxdb-observability/otel2influx v0.0.0-20210429161704-67280d9a921b
	github.com/influxdata/line-protocol/v2 v2.0.0-20210312163703-69fb9462cb3c
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.25.1-0.20210427232103-bfea0f35cec4
	go.uber.org/zap v1.16.0
)
