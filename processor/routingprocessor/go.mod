module github.com/open-telemetry/opentelemetry-collector-contrib/processor/routingprocessor

go 1.16

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/jaegerexporter v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.33.1-0.20210826200354-479f46434f9a
	go.opentelemetry.io/collector/model v0.33.1-0.20210826200354-479f46434f9a
	go.uber.org/zap v1.19.0
	google.golang.org/grpc v1.40.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/jaegerexporter => ../../exporter/jaegerexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger => ../../pkg/translator/jaeger
