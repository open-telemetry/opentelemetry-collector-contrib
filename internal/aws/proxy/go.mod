module github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy

go 1.17

require (
	github.com/aws/aws-sdk-go v1.42.23
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.41.0
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.41.1-0.20211210184707-4dcb3388a168
	go.uber.org/zap v1.19.1

)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.7.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../../internal/coreinternal
