module github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray

go 1.19

require (
	github.com/aws/aws-sdk-go v1.44.245
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil v0.75.0
	github.com/stretchr/testify v1.8.2
	go.opentelemetry.io/collector/component v0.75.0
	go.uber.org/zap v1.24.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/knadh/koanf v1.5.0 // indirect
	github.com/kr/pretty v0.2.1 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/objx v0.5.0 // indirect
	go.opentelemetry.io/collector v0.75.0 // indirect
	go.opentelemetry.io/collector/confmap v0.75.0 // indirect
	go.opentelemetry.io/collector/featuregate v0.75.0 // indirect
	go.opentelemetry.io/otel v1.14.0 // indirect
	go.opentelemetry.io/otel/metric v0.37.0 // indirect
	go.opentelemetry.io/otel/trace v1.14.0 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.9.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil => ../../../internal/aws/awsutil

retract v0.65.0
