module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter

go 1.23.0

require (
	github.com/aws/aws-sdk-go-v2 v1.36.3
	github.com/aws/aws-sdk-go-v2/config v1.29.14
	github.com/aws/aws-sdk-go-v2/credentials v1.17.67
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.17.72
	github.com/aws/aws-sdk-go-v2/service/s3 v1.79.2
	github.com/aws/aws-sdk-go-v2/service/sts v1.33.19
	github.com/itchyny/timefmt-go v0.1.6
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr v0.124.1
	github.com/stretchr/testify v1.10.0
	github.com/tilinna/clock v1.1.0
	go.opentelemetry.io/collector/component v1.30.1-0.20250428165858-4ed72bda40bd
	go.opentelemetry.io/collector/component/componenttest v0.124.1-0.20250428165858-4ed72bda40bd
	go.opentelemetry.io/collector/config/configcompression v1.30.1-0.20250428165858-4ed72bda40bd
	go.opentelemetry.io/collector/confmap v1.30.1-0.20250428165858-4ed72bda40bd
	go.opentelemetry.io/collector/consumer v1.30.1-0.20250428165858-4ed72bda40bd
	go.opentelemetry.io/collector/exporter v0.124.1-0.20250428165858-4ed72bda40bd
	go.opentelemetry.io/collector/exporter/exportertest v0.124.1-0.20250428165858-4ed72bda40bd
	go.opentelemetry.io/collector/otelcol/otelcoltest v0.124.1-0.20250428165858-4ed72bda40bd
	go.opentelemetry.io/collector/pdata v1.30.1-0.20250428165858-4ed72bda40bd
	go.uber.org/goleak v1.3.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.0
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.6.10 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.30 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.34 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.34 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.3 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.3.34 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.12.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.7.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.12.15 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.18.15 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.25.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.30.1 // indirect
	github.com/aws/smithy-go v1.22.2 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.2 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/ebitengine/purego v0.8.2 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.26.1 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.2.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/prometheus/client_golang v1.21.1 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.62.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/shirou/gopsutil/v4 v4.25.3 // indirect
	github.com/spf13/cobra v1.9.1 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/component/componentstatus v0.124.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/config/configretry v1.30.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.124.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/confmap/provider/envprovider v1.30.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/confmap/provider/fileprovider v1.30.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/confmap/provider/httpprovider v1.30.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/confmap/provider/yamlprovider v1.30.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/confmap/xconfmap v0.124.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/connector v0.124.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/connector/connectortest v0.124.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/connector/xconnector v0.124.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.124.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/consumer/consumertest v0.124.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.124.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/exporter/xexporter v0.124.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/extension v1.30.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/extension/extensioncapabilities v0.124.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/extension/extensiontest v0.124.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/extension/xextension v0.124.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/featuregate v1.30.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/internal/fanoutconsumer v0.124.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.124.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/otelcol v0.124.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.124.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/pdata/testdata v0.124.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/pipeline v0.124.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.124.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/processor v1.30.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/processor/processortest v0.124.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/processor/xprocessor v0.124.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/receiver v1.30.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/receiver/receivertest v0.124.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.124.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/semconv v0.124.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/service v0.124.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/collector/service/hostcapabilities v0.124.1-0.20250428165858-4ed72bda40bd // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.10.0 // indirect
	go.opentelemetry.io/contrib/otelconf v0.15.0 // indirect
	go.opentelemetry.io/contrib/propagators/b3 v1.35.0 // indirect
	go.opentelemetry.io/otel v1.35.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc v0.11.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.11.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.35.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.35.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.35.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.35.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.35.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.57.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutlog v0.11.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v1.35.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.35.0 // indirect
	go.opentelemetry.io/otel/log v0.11.0 // indirect
	go.opentelemetry.io/otel/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk/log v0.11.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/trace v1.35.0 // indirect
	go.opentelemetry.io/proto/otlp v1.5.0 // indirect
	golang.org/x/exp v0.0.0-20240506185415-9bf2ced13842 // indirect
	golang.org/x/net v0.39.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	gonum.org/v1/gonum v0.16.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250218202821-56aae31c358a // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250218202821-56aae31c358a // indirect
	google.golang.org/grpc v1.72.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

retract (
	v0.76.2
	v0.76.1
)

replace go.opentelemetry.io/collector/extension/extensionmiddleware/extensionmiddlewaretest v0.0.0-00010101000000-000000000000 => go.opentelemetry.io/collector/extension/extensionmiddleware/extensionmiddlewaretest v0.0.0-20250428165858-4ed72bda40bd

replace go.opentelemetry.io/collector/config/configmiddleware v0.0.0-00010101000000-000000000000 => go.opentelemetry.io/collector/config/configmiddleware v0.0.0-20250428165858-4ed72bda40bd

replace go.opentelemetry.io/collector/extension/extensionmiddleware v1.30.0 => go.opentelemetry.io/collector/extension/extensionmiddleware v0.0.0-20250428165858-4ed72bda40bd

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr => ../../pkg/batchperresourceattr
