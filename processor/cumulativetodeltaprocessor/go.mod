module github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor

go 1.16

require (
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.29.1-0.20210712181854-e69cebab8e0c
	go.opentelemetry.io/collector/model v0.0.0-00010101000000-000000000000
	go.uber.org/zap v1.18.1
)

replace go.opentelemetry.io/collector/model => go.opentelemetry.io/collector/model v0.0.0-20210712181854-e69cebab8e0c
