module github.com/open-telemetry/opentelemetry-collector-contrib/internal/common

go 1.26.0

require (
	github.com/distribution/reference v0.6.0
	github.com/stretchr/testify v1.12.1
	go.opentelemetry.io/collector/featuregate v1.66.0
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.28.0
)

require (
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
)

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)
