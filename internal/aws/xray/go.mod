module github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray

go 1.24.0

require (
	github.com/amazon-contributing/opentelemetry-collector-contrib/override/aws v0.0.0-20250519142056-97a2f5e08ffa
	github.com/aws/aws-sdk-go v1.55.6
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil v0.124.1
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/component v1.30.0
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.1
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.8.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/featuregate v1.49.0 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.124.0 // indirect
	go.opentelemetry.io/collector/pdata v1.49.0 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.10.0 // indirect
	go.opentelemetry.io/otel v1.39.0 // indirect
	go.opentelemetry.io/otel/log v0.11.0 // indirect
	go.opentelemetry.io/otel/metric v1.39.0 // indirect
	go.opentelemetry.io/otel/sdk v1.35.0 // indirect
	go.opentelemetry.io/otel/trace v1.39.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil => ../../../internal/aws/awsutil

replace github.com/amazon-contributing/opentelemetry-collector-contrib/override/aws => ../../../override/aws

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)
