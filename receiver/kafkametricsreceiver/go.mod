module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver

go 1.15

require (
	github.com/Shopify/sarama v1.28.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.25.1-0.20210421230708-d10b842f49eb
	go.uber.org/zap v1.16.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common
