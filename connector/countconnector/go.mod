module github.com/open-telemetry/opentelemetry-collector-contrib/connector/countconnector

go 1.25.0

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter v0.150.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil v0.150.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden v0.150.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl v0.150.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.150.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.150.0
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/component v1.56.0
	go.opentelemetry.io/collector/component/componenttest v0.150.0
	go.opentelemetry.io/collector/confmap v1.56.0
	go.opentelemetry.io/collector/connector v0.150.0
	go.opentelemetry.io/collector/connector/connectortest v0.150.0
	go.opentelemetry.io/collector/connector/xconnector v0.150.0
	go.opentelemetry.io/collector/consumer v1.56.0
	go.opentelemetry.io/collector/consumer/consumertest v0.150.0
	go.opentelemetry.io/collector/otelcol/otelcoltest v0.150.0
	go.opentelemetry.io/collector/pdata v1.56.0
	go.opentelemetry.io/collector/pdata/pprofile v0.150.0
	go.opentelemetry.io/collector/pipeline v1.56.0
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.1
)

require (
	github.com/alecthomas/participle/v2 v2.1.4 // indirect
	github.com/antchfx/xmlquery v1.5.1 // indirect
	github.com/antchfx/xpath v1.3.6 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/ebitengine/purego v0.10.0 // indirect
	github.com/elastic/go-grok v0.3.1 // indirect
	github.com/elastic/lunes v0.2.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/goccy/go-json v0.10.6 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.28.0 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/iancoleman/strcase v0.3.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.4 // indirect
	github.com/lightstep/go-expohisto v1.0.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20251013123823-9fd1530e3ec3 // indirect
	github.com/magefile/mage v1.15.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.150.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/prometheus/client_golang v1.23.2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.67.5 // indirect
	github.com/prometheus/otlptranslator v1.0.0 // indirect
	github.com/prometheus/procfs v0.20.1 // indirect
	github.com/shirou/gopsutil/v4 v4.26.3 // indirect
	github.com/spf13/cobra v1.10.2 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/tklauser/go-sysconf v0.3.16 // indirect
	github.com/tklauser/numcpus v0.11.0 // indirect
	github.com/twmb/murmur3 v1.1.8 // indirect
	github.com/ua-parser/uap-go v0.0.0-20251207011819-db9adb27a0b8 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	github.com/zeebo/xxh3 v1.1.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/client v1.56.0 // indirect
	go.opentelemetry.io/collector/component/componentstatus v0.150.0 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.150.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/envprovider v1.56.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/fileprovider v1.56.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/httpprovider v1.56.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/yamlprovider v1.56.0 // indirect
	go.opentelemetry.io/collector/confmap/xconfmap v0.150.0 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.150.0 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.150.0 // indirect
	go.opentelemetry.io/collector/exporter v1.56.0 // indirect
	go.opentelemetry.io/collector/exporter/exportertest v0.150.0 // indirect
	go.opentelemetry.io/collector/exporter/xexporter v0.150.0 // indirect
	go.opentelemetry.io/collector/extension v1.56.0 // indirect
	go.opentelemetry.io/collector/extension/extensioncapabilities v0.150.0 // indirect
	go.opentelemetry.io/collector/extension/extensiontest v0.150.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.56.0 // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.150.0 // indirect
	go.opentelemetry.io/collector/internal/fanoutconsumer v0.150.0 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.150.0 // indirect
	go.opentelemetry.io/collector/otelcol v0.150.0 // indirect
	go.opentelemetry.io/collector/pdata/testdata v0.150.0 // indirect
	go.opentelemetry.io/collector/pdata/xpdata v0.150.0 // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.150.0 // indirect
	go.opentelemetry.io/collector/processor v1.56.0 // indirect
	go.opentelemetry.io/collector/processor/processortest v0.150.0 // indirect
	go.opentelemetry.io/collector/processor/xprocessor v0.150.0 // indirect
	go.opentelemetry.io/collector/receiver v1.56.0 // indirect
	go.opentelemetry.io/collector/receiver/receivertest v0.150.0 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.150.0 // indirect
	go.opentelemetry.io/collector/service v0.150.0 // indirect
	go.opentelemetry.io/collector/service/hostcapabilities v0.150.0 // indirect
	go.opentelemetry.io/contrib/otelconf v0.23.0 // indirect
	go.opentelemetry.io/otel v1.43.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc v0.19.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.19.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.43.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.43.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.43.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.43.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.43.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.65.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutlog v0.19.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v1.43.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.43.0 // indirect
	go.opentelemetry.io/otel/log v0.19.0 // indirect
	go.opentelemetry.io/otel/metric v1.43.0 // indirect
	go.opentelemetry.io/otel/sdk v1.43.0 // indirect
	go.opentelemetry.io/otel/sdk/log v0.19.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.43.0 // indirect
	go.opentelemetry.io/otel/trace v1.43.0 // indirect
	go.opentelemetry.io/proto/otlp v1.10.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/exp v0.0.0-20260312153236-7ab1446f8b90 // indirect
	golang.org/x/net v0.52.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.36.0 // indirect
	gonum.org/v1/gonum v0.17.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260406210006-6f92a3bedf2d // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260406210006-6f92a3bedf2d // indirect
	google.golang.org/grpc v1.80.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter => ../../internal/filter

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl => ../../pkg/ottl

retract (
	v0.76.2
	v0.76.1
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil => ../../internal/pdatautil
