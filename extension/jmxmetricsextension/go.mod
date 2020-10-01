module github.com/open-telemetry/opentelemetry-collector-contrib/extension/jmxmetricsextension

go 1.14

require (
	github.com/shirou/gopsutil v2.20.6+incompatible
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.11.1-0.20201001213035-035aa5cf6c92
	go.uber.org/atomic v1.7.0
	go.uber.org/zap v1.16.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common
