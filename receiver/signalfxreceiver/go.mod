module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver

go 1.12

require (
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/golang/protobuf v1.3.2
	github.com/gorilla/mux v1.7.3
	github.com/open-telemetry/opentelemetry-collector v0.2.4
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter v0.0.0-20200110233337-37711984b8d4
	github.com/signalfx/com_signalfx_metrics_protobuf v0.0.0-20190530013331-054be550cb49
	github.com/stretchr/testify v1.4.0
	go.opencensus.io v0.22.1
	go.uber.org/zap v1.13.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter => ../../exporter/signalfxexporter
