module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxcorrelationexporter

go 1.14

require (
	github.com/containerd/containerd v1.3.6 // indirect
	github.com/signalfx/signalfx-agent/pkg/apm v0.0.0-20201015185032-52a4f97df2a4
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.13.0
	go.uber.org/zap v1.16.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common
