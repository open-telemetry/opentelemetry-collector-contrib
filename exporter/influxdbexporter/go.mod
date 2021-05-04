module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/influxbexporter

go 1.15

require (
	github.com/influxdata/influxdb-observability/common v0.0.0-20210503044220-4051d4b8738f
	github.com/influxdata/influxdb-observability/otel2influx v0.0.0-20210503044220-4051d4b8738f
	github.com/influxdata/line-protocol/v2 v2.0.0-20210428091617-0567a5134992
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.25.1-0.20210504151019-4603bad78ae7
	go.uber.org/zap v1.16.0
)
