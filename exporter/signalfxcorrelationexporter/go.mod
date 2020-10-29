module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxcorrelationexporter

go 1.14

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk v0.0.0-00010101000000-000000000000
	github.com/signalfx/signalfx-agent/pkg/apm v0.0.0-20201015185032-52a4f97df2a4
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.13.1-0.20201029201142-77f3e6d85f56
	go.uber.org/zap v1.16.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk => ../../internal/splunk
