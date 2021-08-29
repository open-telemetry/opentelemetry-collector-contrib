module github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension

go 1.16

require (
	github.com/jaegertracing/jaeger v1.25.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.33.1-0.20210827152330-09258f969908
	go.uber.org/zap v1.19.0

)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal
