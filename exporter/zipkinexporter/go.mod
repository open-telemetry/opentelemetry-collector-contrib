module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/zipkinexporter

go 1.16

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin v0.0.0-00010101000000-000000000000
	github.com/openzipkin/zipkin-go v0.2.5
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.33.1-0.20210820002854-d3000232f8f6
	go.opentelemetry.io/collector/model v0.33.1-0.20210820002854-d3000232f8f6
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin => ../../pkg/translator/zipkin

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus => ../../pkg/translator/opencensus
