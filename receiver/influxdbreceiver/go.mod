module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/influxdbreceiver

go 1.16

require (
	github.com/gorilla/mux v1.8.0
	github.com/influxdata/influxdb-observability/common v0.1.1
	github.com/influxdata/influxdb-observability/influx2otel v0.1.1
	github.com/influxdata/line-protocol/v2 v2.0.0-20210520103755-6551a972d603
	go.opentelemetry.io/collector v0.30.2-0.20210719230137-809cae954ed3
	go.opentelemetry.io/collector/model v0.30.2-0.20210719230137-809cae954ed3
	go.uber.org/zap v1.18.1
)
