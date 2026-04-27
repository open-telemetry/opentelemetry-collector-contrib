//module github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azure

go 1.25.0

require (
	github.com/goccy/go-json v0.10.6
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.150.0
	github.com/relvacode/iso8601 v1.7.0
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/component v1.56.1-0.20260424074859-d91c0edd1da5
	go.opentelemetry.io/collector/pdata v1.56.1-0.20260424074859-d91c0edd1da5
	go.opentelemetry.io/otel v1.43.0
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.1
	golang.org/x/exp v0.0.0-20240506185415-9bf2ced13842
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.150.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.56.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/pdata/xpdata v0.150.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/otel/metric v1.43.0 // indirect
	go.opentelemetry.io/otel/trace v1.43.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pdatatest

module github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azure

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../golden
