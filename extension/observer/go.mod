module github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer

go 1.26.0

require (
	github.com/stretchr/testify v1.12.1
	go.uber.org/zap v1.28.0
)

require (
	github.com/stretchr/objx v0.5.3 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
)

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)
