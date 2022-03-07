module github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy

go 1.17

require (
	github.com/aws/aws-sdk-go v1.43.12
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.46.0
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.46.0
	go.uber.org/zap v1.21.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/kr/pretty v0.3.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../../internal/common
