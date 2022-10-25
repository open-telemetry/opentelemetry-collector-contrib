module github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy

go 1.18

require (
	github.com/aws/aws-sdk-go v1.44.122
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.62.0
	github.com/stretchr/testify v1.8.1
	go.opentelemetry.io/collector v0.62.2-0.20221017171445-6313054b642c
	go.uber.org/zap v1.23.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/kr/pretty v0.3.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../../internal/common
