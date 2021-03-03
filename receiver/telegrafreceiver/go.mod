module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/telegrafreceiver

go 1.14

require (
	github.com/influxdata/telegraf v1.17.3
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.21.0
	go.uber.org/zap v1.16.0
)

replace github.com/influxdata/telegraf => github.com/sumologic/telegraf v1.17.3-sumo
