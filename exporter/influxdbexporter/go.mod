module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/influxbexporter

go 1.14

require (
	github.com/gogo/protobuf v1.3.2
	github.com/influxdata/influxdb-observability/common v0.0.0-20210406213806-f0f821b11dc7
	github.com/influxdata/influxdb-observability/otel2influx v0.0.0-20210406213806-f0f821b11dc7
	github.com/influxdata/influxdb-observability/otlp v0.0.0-20210407212709-b0c2966fa83c
	github.com/influxdata/line-protocol/v2 v2.0.0-20210312163703-69fb9462cb3c
	go.opentelemetry.io/collector v0.23.1-0.20210329172532-38e57614135f
	go.uber.org/zap v1.16.0
)
