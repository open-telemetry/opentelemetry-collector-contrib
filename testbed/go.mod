module github.com/open-telemetry/opentelemetry-collector-contrib/testbed

go 1.12

require (
	github.com/google/addlicense v0.0.0-20190907113143-be125746c2c4 // indirect
	github.com/open-telemetry/opentelemetry-collector v0.2.3
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver v0.0.0
	github.com/open-telemetry/opentelemetry-collector/testbed v0.0.0-20200110193345-9f801c5b8d9f
	go.uber.org/zap v1.13.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter => ../exporter/sapmexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter => ../exporter/signalfxexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver => ../receiver/sapmreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver => ../receiver/signalfxreceiver
