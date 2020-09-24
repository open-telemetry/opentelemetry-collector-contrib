module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter

go 1.14

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.0.0-00010101000000-000000000000
	github.com/signalfx/sapm-proto v0.5.3
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.11.0
	go.uber.org/zap v1.16.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common
