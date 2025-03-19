module github.com/open-telemetry/opentelemetry-collector-contrib/internal/common

go 1.23.0

require (
	github.com/distribution/reference v0.6.0
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/featuregate v1.28.0
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)
