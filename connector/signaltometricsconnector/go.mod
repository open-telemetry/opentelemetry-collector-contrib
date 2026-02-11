module github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector

go 1.25.0

require (
	github.com/google/go-cmp v0.7.0
	github.com/lightstep/go-expohisto v1.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden v0.145.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl v0.145.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.145.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.145.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling v0.145.0
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/component v1.51.1-0.20260205185216-81bc641f26c0
	go.opentelemetry.io/collector/component/componenttest v0.145.1-0.20260205185216-81bc641f26c0
	go.opentelemetry.io/collector/config/configoptional v1.51.1-0.20260205185216-81bc641f26c0
	go.opentelemetry.io/collector/confmap v1.51.1-0.20260205185216-81bc641f26c0
	go.opentelemetry.io/collector/confmap/xconfmap v0.145.1-0.20260205185216-81bc641f26c0
	go.opentelemetry.io/collector/connector v0.145.1-0.20260205185216-81bc641f26c0
	go.opentelemetry.io/collector/connector/connectortest v0.145.1-0.20260205185216-81bc641f26c0
	go.opentelemetry.io/collector/connector/xconnector v0.145.1-0.20260205185216-81bc641f26c0
	go.opentelemetry.io/collector/consumer v1.51.1-0.20260205185216-81bc641f26c0
	go.opentelemetry.io/collector/consumer/consumertest v0.145.1-0.20260205185216-81bc641f26c0
	go.opentelemetry.io/collector/pdata v1.51.1-0.20260205185216-81bc641f26c0
	go.opentelemetry.io/collector/pdata/pprofile v0.145.1-0.20260205185216-81bc641f26c0
	go.opentelemetry.io/collector/pipeline v1.51.1-0.20260205185216-81bc641f26c0
	go.opentelemetry.io/otel v1.40.0
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.1
)

require (
	github.com/alecthomas/participle/v2 v2.1.4 // indirect
	github.com/antchfx/xmlquery v1.5.0 // indirect
	github.com/antchfx/xpath v1.3.5 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/elastic/go-grok v0.3.1 // indirect
	github.com/elastic/lunes v0.2.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.8.0 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/iancoleman/strcase v0.3.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.2.10 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.2 // indirect
	github.com/magefile/mage v1.15.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.145.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/twmb/murmur3 v1.1.8 // indirect
	github.com/ua-parser/uap-go v0.0.0-20240611065828-3a4781585db6 // indirect
	github.com/zeebo/xxh3 v1.1.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/featuregate v1.51.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/internal/fanoutconsumer v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/otel/metric v1.40.0 // indirect
	go.opentelemetry.io/otel/sdk v1.40.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.40.0 // indirect
	go.opentelemetry.io/otel/trace v1.40.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/exp v0.0.0-20240506185415-9bf2ced13842 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl => ../../pkg/ottl

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling => ../../pkg/sampling
