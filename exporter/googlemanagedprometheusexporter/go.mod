module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlemanagedprometheusexporter

go 1.25.0

require (
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector v0.56.0
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/googlemanagedprometheus v0.56.0
	github.com/prometheus/otlptranslator v1.0.0
	github.com/prometheus/prometheus v0.311.2-0.20260409145810-72293ff1d2e0
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/component v1.56.1-0.20260424074859-d91c0edd1da5
	go.opentelemetry.io/collector/component/componenttest v0.150.1-0.20260424074859-d91c0edd1da5
	go.opentelemetry.io/collector/config/configoptional v1.56.1-0.20260424074859-d91c0edd1da5
	go.opentelemetry.io/collector/confmap v1.56.1-0.20260424074859-d91c0edd1da5
	go.opentelemetry.io/collector/consumer v1.56.1-0.20260424074859-d91c0edd1da5
	go.opentelemetry.io/collector/exporter v1.56.1-0.20260424074859-d91c0edd1da5
	go.opentelemetry.io/collector/exporter/exporterhelper v0.150.1-0.20260424074859-d91c0edd1da5
	go.opentelemetry.io/collector/exporter/exportertest v0.150.1-0.20260424074859-d91c0edd1da5
	go.opentelemetry.io/collector/otelcol/otelcoltest v0.150.1-0.20260424074859-d91c0edd1da5
	go.opentelemetry.io/collector/pdata v1.56.1-0.20260424074859-d91c0edd1da5
	go.uber.org/goleak v1.3.0
)

require (
	cloud.google.com/go v0.123.0 // indirect
	cloud.google.com/go/auth v0.18.2 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	cloud.google.com/go/logging v1.13.2 // indirect
	cloud.google.com/go/longrunning v0.8.0 // indirect
	cloud.google.com/go/monitoring v1.24.3 // indirect
	cloud.google.com/go/trace v1.11.7 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.21.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.13.1 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.11.2 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.6.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v1.32.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.56.0 // indirect
	github.com/alecthomas/units v0.0.0-20240927000941-0f3dac36c52b // indirect
	github.com/aws/aws-sdk-go-v2 v1.41.4 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.32.12 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.19.12 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.20 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.20 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.20 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.20 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.0.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.30.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.35.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.41.9 // indirect
	github.com/aws/smithy-go v1.24.2 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dennwc/varint v1.0.0 // indirect
	github.com/ebitengine/purego v0.10.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.14 // indirect
	github.com/googleapis/gax-go/v2 v2.18.0 // indirect
	github.com/grafana/regexp v0.0.0-20250905093917-f7b3be9d1853 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.28.0 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.18.5 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.4 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20251013123823-9fd1530e3ec3 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/prometheus/client_golang v1.23.2 // indirect
	github.com/prometheus/client_golang/exp v0.0.0-20260325093428-d8591d0db856 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.67.5 // indirect
	github.com/prometheus/procfs v0.20.1 // indirect
	github.com/prometheus/sigv4 v0.4.1 // indirect
	github.com/shirou/gopsutil/v4 v4.26.3 // indirect
	github.com/spf13/cobra v1.10.2 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.2.0 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/tinylru v1.2.1 // indirect
	github.com/tidwall/wal v1.2.1 // indirect
	github.com/tklauser/go-sysconf v0.3.16 // indirect
	github.com/tklauser/numcpus v0.11.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/client v1.56.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/component/componentstatus v0.150.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/config/configretry v1.56.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.150.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/confmap/provider/envprovider v1.56.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/confmap/provider/fileprovider v1.56.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/confmap/provider/httpprovider v1.56.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/confmap/provider/yamlprovider v1.56.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/confmap/xconfmap v0.150.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/connector v0.150.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/connector/connectortest v0.150.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/connector/xconnector v0.150.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.150.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/consumer/consumertest v0.150.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.150.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/exporter/xexporter v0.150.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/extension v1.56.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/extension/extensioncapabilities v0.150.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/extension/extensiontest v0.150.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/extension/xextension v0.150.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/featuregate v1.56.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.150.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/internal/fanoutconsumer v0.150.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.150.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/otelcol v0.150.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.150.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/pdata/testdata v0.150.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/pdata/xpdata v0.150.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/pipeline v1.56.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.150.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/processor v1.56.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/processor/processortest v0.150.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/processor/xprocessor v0.150.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/receiver v1.56.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/receiver/receivertest v0.150.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.150.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/service v0.150.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/collector/service/hostcapabilities v0.150.1-0.20260424074859-d91c0edd1da5 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.63.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.68.0 // indirect
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
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.1 // indirect
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.50.0 // indirect
	golang.org/x/exp v0.0.0-20260312153236-7ab1446f8b90 // indirect
	golang.org/x/net v0.53.0 // indirect
	golang.org/x/oauth2 v0.36.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/text v0.36.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	gonum.org/v1/gonum v0.17.0 // indirect
	google.golang.org/api v0.272.0 // indirect
	google.golang.org/genproto v0.0.0-20260217215200-42d3e9bedb6d // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260406210006-6f92a3bedf2d // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260406210006-6f92a3bedf2d // indirect
	google.golang.org/grpc v1.80.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apimachinery v0.35.3 // indirect
	k8s.io/client-go v0.35.3 // indirect
	k8s.io/klog/v2 v2.140.0 // indirect
	k8s.io/utils v0.0.0-20251002143259-bc988d571ff4 // indirect
)

retract (
	v0.88.0
	v0.87.0
	v0.86.0
	v0.85.0
	v0.84.0
	v0.76.2
	v0.76.1
	v0.65.0
)
