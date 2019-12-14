module github.com/open-telemetry/opentelemetry-collector-contrib/testbed

go 1.12

require (
	github.com/open-telemetry/opentelemetry-collector v0.2.1-0.20191213162008-563eac85a88c
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter v0.0.0-20191213162202-55b8658da81a
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver v0.0.0-20191213162202-55b8658da81a
	github.com/open-telemetry/opentelemetry-collector/testbed v0.0.0-20191213162008-563eac85a88c
	go.uber.org/zap v1.13.0
)
