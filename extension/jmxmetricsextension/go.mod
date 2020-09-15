module github.com/open-telemetry/opentelemetry-collector-contrib/extension/jmxmetricsextension

go 1.14

require (
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.10.1-0.20200915193938-b3a5ceaefa96
	go.uber.org/zap v1.16.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common
