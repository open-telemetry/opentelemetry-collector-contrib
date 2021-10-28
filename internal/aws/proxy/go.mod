module github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy

go 1.17

require (
	github.com/aws/aws-sdk-go v1.41.12
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.37.1
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.37.1-0.20211026180946-46c8e2290e45
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
