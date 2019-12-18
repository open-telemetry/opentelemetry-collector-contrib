module github.com/open-telemetry/opentelemetry-collector-contrib/testbed

go 1.12

require (
	github.com/aws/aws-sdk-go v1.23.20 // indirect
	github.com/google/addlicense v0.0.0-20190907113143-be125746c2c4 // indirect
	github.com/open-telemetry/opentelemetry-collector v0.2.1-0.20191217213608-3cac55bf869f
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter v0.0.0-20191217233418-a191cedc5fcb
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter v0.0.0-20191217233418-a191cedc5fcb
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver v0.0.0-20191217233418-a191cedc5fcb
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver v0.0.0-20191217233418-a191cedc5fcb
	github.com/open-telemetry/opentelemetry-collector/testbed v0.0.0-20191217213608-3cac55bf869f
	go.uber.org/zap v1.13.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter => ../exporter/sapmexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter => ../exporter/signalfxexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver => ../receiver/sapmreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver => ../receiver/signalfxreceiver
