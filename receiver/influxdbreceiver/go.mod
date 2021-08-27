module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/influxdbreceiver

go 1.16

require (
	github.com/influxdata/influxdb-observability/common v0.2.5
	github.com/influxdata/influxdb-observability/influx2otel v0.2.5
	github.com/influxdata/line-protocol/v2 v2.0.0-20210520103755-6551a972d603
	go.opentelemetry.io/collector v0.33.1-0.20210827152330-09258f969908
	go.opentelemetry.io/collector/model v0.33.1-0.20210827152330-09258f969908 // indirect
	go.uber.org/zap v1.19.0
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/tools v0.1.3 // indirect
)
