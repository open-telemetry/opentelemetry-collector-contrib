module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitlabreceiver

go 1.26.0

require (
	github.com/stretchr/testify v1.12.1
	gitlab.com/gitlab-org/api/client-go/v3 v3.45.0
	go.opentelemetry.io/collector/component v1.71.0
	go.opentelemetry.io/collector/component/componenttest v0.165.0
	go.opentelemetry.io/collector/config/confighttp v1.71.0
	go.opentelemetry.io/collector/config/confignet v1.71.0
	go.opentelemetry.io/collector/confmap v1.71.0
	go.opentelemetry.io/collector/consumer v1.71.0
	go.opentelemetry.io/collector/consumer/consumertest v0.165.0
	go.opentelemetry.io/collector/receiver v1.71.0
	go.opentelemetry.io/collector/receiver/receiverhelper v0.165.0
	go.opentelemetry.io/collector/receiver/receivertest v0.165.0
	go.uber.org/goleak v1.3.0
)

require (
	github.com/Microsoft/go-winio v0.6.4 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/ebitengine/purego v0.11.2 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20260902202739-8c9c2d1005f4 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/google/go-querystring v1.2.0 // indirect
	github.com/google/go-tpm v0.9.9-0.20260124013517-8f8f42cba0de // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.31.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.8 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/lufia/plan9stats v0.0.0-20251013123823-9fd1530e3ec3 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/power-devops/perfstat v0.0.0-20260805114148-88456608a4f6 // indirect
	github.com/prometheus/client_golang v1.24.1 // indirect
	github.com/prometheus/client_model v0.6.3 // indirect
	github.com/prometheus/common v0.72.1 // indirect
	github.com/prometheus/otlptranslator v1.0.0 // indirect
	github.com/prometheus/procfs v0.22.0 // indirect
	github.com/shirou/gopsutil/v4 v4.26.10 // indirect
	github.com/spf13/cobra v1.10.2 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/tklauser/go-sysconf v0.4.0 // indirect
	github.com/tklauser/numcpus v0.12.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/config/configmiddleware v1.71.0 // indirect
	go.opentelemetry.io/collector/config/configoptional v1.71.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/envprovider v1.71.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/fileprovider v1.71.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/httpprovider v1.71.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/yamlprovider v1.71.0 // indirect
	go.opentelemetry.io/collector/confmap/xconfmap v0.165.0 // indirect
	go.opentelemetry.io/collector/connector v0.165.0 // indirect
	go.opentelemetry.io/collector/connector/connectortest v0.165.0 // indirect
	go.opentelemetry.io/collector/connector/xconnector v0.165.0 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.165.0 // indirect
	go.opentelemetry.io/collector/exporter v1.71.0 // indirect
	go.opentelemetry.io/collector/exporter/exportertest v0.165.0 // indirect
	go.opentelemetry.io/collector/exporter/xexporter v0.165.0 // indirect
	go.opentelemetry.io/collector/extension/extensionauth v1.71.0 // indirect
	go.opentelemetry.io/collector/extension/extensioncapabilities v0.165.0 // indirect
	go.opentelemetry.io/collector/extension/extensionmiddleware v0.165.0 // indirect
	go.opentelemetry.io/collector/extension/extensiontest v0.165.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.71.0 // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.165.0 // indirect
	go.opentelemetry.io/collector/internal/fanoutconsumer v0.165.0 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.165.0 // indirect
	go.opentelemetry.io/collector/otelcol v0.165.0 // indirect
	go.opentelemetry.io/collector/pdata/testdata v0.165.0 // indirect
	go.opentelemetry.io/collector/pdata/xpdata v0.165.0 // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.165.0 // indirect
	go.opentelemetry.io/collector/processor v1.71.0 // indirect
	go.opentelemetry.io/collector/processor/processortest v0.165.0 // indirect
	go.opentelemetry.io/collector/processor/xprocessor v0.165.0 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.165.0 // indirect
	go.opentelemetry.io/collector/service v0.165.0 // indirect
	go.opentelemetry.io/collector/service/hostcapabilities v0.165.0 // indirect
	go.opentelemetry.io/contrib/otelconf v0.27.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc v0.23.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.23.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.69.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutlog v0.23.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.47.0 // indirect
	go.opentelemetry.io/otel/log v1.47.0 // indirect
	go.opentelemetry.io/otel/sdk/log v1.47.0 // indirect
	go.opentelemetry.io/proto/otlp v1.11.1 // indirect
	go.yaml.in/yaml/v3 v3.0.6 // indirect
	golang.org/x/crypto v0.58.0 // indirect
	golang.org/x/exp v0.0.0-20260908205506-85c1c2202aba // indirect
	golang.org/x/oauth2 v0.37.0 // indirect
	golang.org/x/time v0.16.0 // indirect
	gonum.org/v1/gonum v0.17.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260928230214-8a89bd6388cc // indirect
)

require (
	github.com/felixge/httpsnoop v1.1.0 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/klauspost/compress v1.20.3 // indirect
	github.com/knadh/koanf/maps v0.1.3 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.1 // indirect
	github.com/knadh/koanf/v2 v2.4.1 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/pierrec/lz4/v4 v4.1.33 // indirect
	github.com/rs/cors v1.11.1 // indirect
	go.opentelemetry.io/collector/client v1.71.0 // indirect
	go.opentelemetry.io/collector/component/componentstatus v0.165.0
	go.opentelemetry.io/collector/config/configauth v1.71.0 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.71.0 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.71.0
	go.opentelemetry.io/collector/config/configtelemetry v0.165.0 // indirect
	go.opentelemetry.io/collector/config/configtls v1.71.0 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.165.0 // indirect
	go.opentelemetry.io/collector/extension v1.71.0 // indirect
	go.opentelemetry.io/collector/otelcol/otelcoltest v0.165.0
	go.opentelemetry.io/collector/pdata v1.71.0
	go.opentelemetry.io/collector/pdata/pprofile v0.165.0 // indirect
	go.opentelemetry.io/collector/pipeline v1.71.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.72.0 // indirect
	go.opentelemetry.io/otel v1.47.0
	go.opentelemetry.io/otel/metric v1.47.0 // indirect
	go.opentelemetry.io/otel/sdk v1.47.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.47.0 // indirect
	go.opentelemetry.io/otel/trace v1.47.0 // indirect
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.28.0
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260928230214-8a89bd6388cc // indirect
	google.golang.org/grpc v1.85.0 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)
