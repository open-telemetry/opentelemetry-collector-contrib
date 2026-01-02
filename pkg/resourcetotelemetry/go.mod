module github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry

go 1.24.0

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.142.0
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/consumer v1.48.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/exporter v1.48.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/pdata v1.48.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/otel v1.39.0
	go.uber.org/goleak v1.3.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/hashicorp/go-version v1.8.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opentelemetry.io/collector/component v1.48.1-0.20251231114627-ad49dc64648d // indirect
	go.opentelemetry.io/collector/featuregate v1.48.1-0.20251231114627-ad49dc64648d // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.142.1-0.20251231114627-ad49dc64648d // indirect
	go.opentelemetry.io/collector/pipeline v1.48.1-0.20251231114627-ad49dc64648d // indirect
	go.opentelemetry.io/otel/metric v1.39.0 // indirect
	go.opentelemetry.io/otel/trace v1.39.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../golden
