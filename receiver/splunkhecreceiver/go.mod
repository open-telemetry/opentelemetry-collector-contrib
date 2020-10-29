module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver

go 1.14

require (
	github.com/gorilla/mux v1.8.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.6.1
	go.opencensus.io v0.22.4
	go.opentelemetry.io/collector v0.13.1-0.20201029001237-1d38caf3b757
	go.uber.org/zap v1.16.0
	google.golang.org/grpc/examples v0.0.0-20200728194956-1c32b02682df // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk => ../../internal/splunk
