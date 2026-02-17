module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter

go 1.25.0

require (
	github.com/cespare/xxhash/v2 v2.3.0
	github.com/elastic/elastic-transport-go/v8 v8.8.0
	github.com/elastic/go-docappender/v2 v2.12.1
	github.com/elastic/go-freelru v0.16.0
	github.com/elastic/go-structform v0.0.12
	github.com/klauspost/compress v1.18.4
	github.com/lestrrat-go/strftime v1.1.1
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.145.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.145.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.145.0
	github.com/stretchr/testify v1.11.1
	github.com/tidwall/gjson v1.18.0
	go.opentelemetry.io/collector/client v1.51.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/component v1.51.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/component/componentstatus v0.145.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/component/componenttest v0.145.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/config/configauth v1.51.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/config/configcompression v1.51.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/config/confighttp v0.145.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/config/configopaque v1.51.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/config/configoptional v1.51.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/confmap v1.51.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/confmap/xconfmap v0.145.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/consumer/consumererror v0.145.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/exporter v1.51.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/exporter/exporterhelper v0.145.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/exporter/exporterhelper/xexporterhelper v0.145.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/exporter/exportertest v0.145.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/exporter/xexporter v0.145.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/extension v1.51.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/extension/extensionauth v1.51.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/pdata v1.51.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/pdata/pprofile v0.145.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/ebpf-profiler v0.0.202606
	go.opentelemetry.io/otel v1.40.0
	go.opentelemetry.io/otel/metric v1.40.0
	go.opentelemetry.io/otel/sdk/metric v1.40.0
	go.opentelemetry.io/otel/trace v1.40.0
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.1
	golang.org/x/sync v0.19.0
)

require (
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cilium/ebpf v0.20.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/elastic/go-sysinfo v1.15.3 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20251226215517-609e4778396f // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/go-tpm v0.9.8 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.8.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.2 // indirect
	github.com/minio/sha256-simd v1.0.1 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.145.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.25 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/procfs v0.16.0 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	go.elastic.co/fastjson v1.5.1 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/config/configmiddleware v1.51.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/config/confignet v1.51.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/config/configretry v1.51.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/config/configtls v1.51.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/consumer v1.51.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/consumer/consumererror/xconsumererror v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/consumer/consumertest v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/extension/extensionmiddleware v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/extension/xextension v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/featuregate v1.51.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/pdata/xpdata v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/pipeline v1.51.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/receiver v1.51.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/receiver/receivertest v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.63.0 // indirect
	go.opentelemetry.io/otel/sdk v1.40.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.48.0 // indirect
	golang.org/x/net v0.50.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251222181119-0a764e51fe1b // indirect
	google.golang.org/grpc v1.79.1 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	howett.net/plist v1.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden
