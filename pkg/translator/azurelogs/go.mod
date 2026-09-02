module github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azurelogs

go 1.26.0

require (
	github.com/goccy/go-json v0.10.6
	github.com/json-iterator/go v1.1.12
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden v0.160.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.160.0
	github.com/relvacode/iso8601 v1.8.0
	github.com/stretchr/testify v1.12.1
	go.opentelemetry.io/collector/component v1.66.0
	go.opentelemetry.io/collector/featuregate v1.66.0
	go.opentelemetry.io/collector/pdata v1.66.0
	go.opentelemetry.io/otel v1.46.0
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.28.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.160.0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.160.0 // indirect
	go.opentelemetry.io/collector/pdata/xpdata v0.160.0 // indirect
	go.opentelemetry.io/otel/metric v1.46.0 // indirect
	go.opentelemetry.io/otel/trace v1.46.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../golden
