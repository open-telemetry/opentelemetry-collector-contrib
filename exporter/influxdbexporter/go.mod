module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/influxdbexporter

go 1.16

require (
	github.com/influxdata/influxdb-observability/common v0.1.0
	github.com/influxdata/influxdb-observability/otel2influx v0.1.0
	github.com/influxdata/line-protocol/v2 v2.0.0-20210428091617-0567a5134992
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.29.0
	go.uber.org/zap v1.17.0
)
