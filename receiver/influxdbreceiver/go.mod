module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/influxdbreceiver

go 1.15

require (
	github.com/gorilla/mux v1.8.0
	github.com/influxdata/influxdb-observability/common v0.0.0-20210503044220-4051d4b8738f
	github.com/influxdata/influxdb-observability/influx2otel v0.0.0-20210503044220-4051d4b8738f
	github.com/influxdata/line-protocol/v2 v2.0.0-20210428091617-0567a5134992
	go.opentelemetry.io/collector v0.27.1-0.20210524201935-86ea0a131fb2
	go.uber.org/zap v1.16.0
)
