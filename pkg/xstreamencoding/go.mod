module github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xstreamencoding

go 1.25.0

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding v0.145.0
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/pdata v1.51.1-0.20260212054546-f0da990367b6
	go.uber.org/goleak v1.3.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/hashicorp/go-version v1.8.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	go.opentelemetry.io/collector/component v1.51.1-0.20260212054546-f0da990367b6 // indirect
	go.opentelemetry.io/collector/extension v1.51.1-0.20260212054546-f0da990367b6 // indirect
	go.opentelemetry.io/collector/featuregate v1.51.1-0.20260212054546-f0da990367b6 // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.145.1-0.20260212054546-f0da990367b6 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.145.1-0.20260212054546-f0da990367b6 // indirect
	go.opentelemetry.io/otel v1.40.0 // indirect
	go.opentelemetry.io/otel/metric v1.40.0 // indirect
	go.opentelemetry.io/otel/trace v1.40.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding => ../../extension/encoding
