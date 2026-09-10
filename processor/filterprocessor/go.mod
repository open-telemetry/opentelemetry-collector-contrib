module github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor

go 1.26.0

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.160.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter v0.160.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl v0.160.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.160.0
	github.com/stretchr/testify v1.12.1
	go.opentelemetry.io/collector/component v1.66.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/component/componenttest v0.160.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/confmap v1.66.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/consumer v1.66.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/consumer/consumertest v0.160.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/consumer/xconsumer v0.160.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/featuregate v1.66.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/pdata v1.66.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/pdata/pprofile v0.160.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/pipeline v1.66.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/pipeline/xpipeline v0.160.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/processor v1.66.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/processor/processorhelper v0.160.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/processor/processorhelper/xprocessorhelper v0.160.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/processor/processortest v0.160.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/processor/xprocessor v0.160.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/otel v1.46.0
	go.opentelemetry.io/otel/metric v1.46.0
	go.opentelemetry.io/otel/sdk/metric v1.46.0
	go.opentelemetry.io/otel/trace v1.46.0
	go.uber.org/goleak v1.3.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.28.0
)

require (
	github.com/alecthomas/participle/v2 v2.1.4 // indirect
	github.com/antchfx/xmlquery v1.5.1 // indirect
	github.com/antchfx/xpath v1.3.8 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/elastic/go-grok v0.3.1 // indirect
	github.com/elastic/lunes v0.2.2 // indirect
	github.com/expr-lang/expr v1.17.8 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/goccy/go-json v0.10.6 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/iancoleman/strcase v0.3.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/knadh/koanf/maps v0.1.3 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.1 // indirect
	github.com/knadh/koanf/v2 v2.3.6 // indirect
	github.com/magefile/mage v1.15.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.160.0 // indirect
	github.com/twmb/murmur3 v1.1.8 // indirect
	github.com/ua-parser/uap-go v0.0.0-20251207011819-db9adb27a0b8 // indirect
	github.com/zeebo/xxh3 v1.1.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/client v1.66.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/component/componentstatus v0.160.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.160.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/pdata/testdata v0.160.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/pdata/xpdata v0.160.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/otel/sdk v1.46.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/exp v0.0.0-20260218203240-3dfff04db8fa // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/grpc v1.83.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter => ../../internal/filter

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl => ../../pkg/ottl

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden
