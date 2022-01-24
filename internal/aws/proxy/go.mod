module github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy

go 1.17

require (
	github.com/aws/aws-sdk-go v1.42.39
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.42.0
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.42.1-0.20220121210129-2c5eb7ca1ad5
	go.uber.org/zap v1.20.0

)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rogpeppe/go-internal v1.8.1 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.7.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../../internal/coreinternal
