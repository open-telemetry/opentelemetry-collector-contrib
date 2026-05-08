module github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/jaegerencodingextension

go 1.25.0

require (
	github.com/gogo/protobuf v1.3.2
	github.com/jaegertracing/jaeger-idl v0.6.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding v0.151.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger v0.151.0
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/component v1.57.1-0.20260501001745-24aecacf1c04
	go.opentelemetry.io/collector/component/componenttest v0.151.1-0.20260501001745-24aecacf1c04
	go.opentelemetry.io/collector/confmap v1.57.1-0.20260501001745-24aecacf1c04
	go.opentelemetry.io/collector/extension v1.57.1-0.20260501001745-24aecacf1c04
	go.opentelemetry.io/collector/extension/extensiontest v0.151.1-0.20260501001745-24aecacf1c04
	go.opentelemetry.io/collector/pdata v1.57.1-0.20260501001745-24aecacf1c04
	go.uber.org/goleak v1.3.0
)

require (
	github.com/apache/thrift v0.23.1-0.20260429145742-d2acd3c49e58 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.4 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.151.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/core/xidutils v0.151.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/featuregate v1.57.1-0.20260501001745-24aecacf1c04 // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.151.1-0.20260501001745-24aecacf1c04 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.151.1-0.20260501001745-24aecacf1c04 // indirect
	go.opentelemetry.io/otel v1.43.0 // indirect
	go.opentelemetry.io/otel/metric v1.43.0 // indirect
	go.opentelemetry.io/otel/sdk v1.43.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.43.0 // indirect
	go.opentelemetry.io/otel/trace v1.43.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.28.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/sys v0.43.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/core/xidutils => ../../../pkg/core/xidutils

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger => ../../../pkg/translator/jaeger

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../../pkg/golden

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding => ../
