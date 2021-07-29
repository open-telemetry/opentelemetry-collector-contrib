module github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor

go 1.16

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.30.2-0.20210727185145-88b2935343aa
	go.opentelemetry.io/collector/model v0.30.2-0.20210727185145-88b2935343aa
	go.uber.org/zap v1.18.1
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics => ./../../internal/aws/metrics
