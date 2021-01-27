module github.com/opentelemetry/opentelemetry-log-collection/operator/builtin/output/otlp

go 1.14

require (
	github.com/mitchellh/mapstructure v1.3.2
	github.com/opentelemetry/opentelemetry-log-collection v0.13.12
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.13.0
	go.uber.org/zap v1.16.0
	gopkg.in/yaml.v2 v2.3.0
)

replace github.com/opentelemetry/opentelemetry-log-collection => ../../../../
