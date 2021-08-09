module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/influxdbreceiver

go 1.16

require (
	github.com/influxdata/influxdb-observability/common v0.2.4
	github.com/influxdata/influxdb-observability/influx2otel v0.2.4
	github.com/influxdata/line-protocol/v2 v2.0.0-20210520103755-6551a972d603
	go.opentelemetry.io/collector v0.31.1-0.20210807221137-acd1eb198b27
	go.opentelemetry.io/collector/model v0.31.1-0.20210807221137-acd1eb198b27 // indirect
	go.uber.org/zap v1.19.0
)
