module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter

go 1.16

require (
	github.com/Shopify/sarama v1.29.1
	github.com/gogo/protobuf v1.3.2
	github.com/jaegertracing/jaeger v1.25.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.7.0
	github.com/xdg-go/scram v1.0.2
	go.opentelemetry.io/collector v0.33.1-0.20210820002854-d3000232f8f6
	go.opentelemetry.io/collector/model v0.33.1-0.20210820002854-d3000232f8f6
	go.uber.org/zap v1.19.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal
