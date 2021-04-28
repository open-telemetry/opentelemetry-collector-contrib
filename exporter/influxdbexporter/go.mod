module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/influxbexporter

go 1.15

require (
	github.com/influxdata/influxdb-observability/common v0.0.0-20210428231654-7754b0dcad5e
	github.com/influxdata/influxdb-observability/otel2influx v0.0.0-20210428231654-7754b0dcad5e
	github.com/influxdata/line-protocol/v2 v2.0.0-20210312163703-69fb9462cb3c
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.25.0
	go.uber.org/zap v1.16.0
)
