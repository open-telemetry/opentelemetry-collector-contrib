module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxmetricsreceiver

go 1.14

require (
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.8.0
	go.uber.org/zap v1.15.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common
