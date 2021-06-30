module github.com/open-telemetry/opentelemetry-collector-contrib/testbed/mockdatareceivers/mockawsxrayreceiver

go 1.16

require (
	github.com/gorilla/mux v1.8.0
	go.opentelemetry.io/collector v0.29.1-0.20210630154722-7d0a0398174e
	go.opentelemetry.io/collector/model v0.0.0-00010101000000-000000000000
	go.uber.org/zap v1.18.1
)

replace go.opentelemetry.io/collector/model => go.opentelemetry.io/collector/model v0.0.0-20210630154722-7d0a0398174e
