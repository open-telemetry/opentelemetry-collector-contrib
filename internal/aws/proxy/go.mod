module github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy

go 1.19

require (
	github.com/aws/aws-sdk-go v1.44.271
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.78.0
	github.com/stretchr/testify v1.8.4
	go.opentelemetry.io/collector v0.78.3-0.20230601234953-deffd4892002
	go.uber.org/zap v1.24.0
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.0.0-rcv0012.0.20230601234953-deffd4892002 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/sys v0.8.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../../internal/common

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)
