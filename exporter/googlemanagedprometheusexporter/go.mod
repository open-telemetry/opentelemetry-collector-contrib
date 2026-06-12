module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlemanagedprometheusexporter

go 1.25.0

require (
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector v0.57.0
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/googlemanagedprometheus v0.57.0
	github.com/prometheus/otlptranslator v1.0.0
	github.com/prometheus/prometheus v0.312.0
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/component v1.60.1-0.20260611154946-8ae0933981eb
	go.opentelemetry.io/collector/component/componenttest v0.154.1-0.20260611154946-8ae0933981eb
	go.opentelemetry.io/collector/config/configoptional v1.60.1-0.20260611154946-8ae0933981eb
	go.opentelemetry.io/collector/confmap v1.60.1-0.20260611154946-8ae0933981eb
	go.opentelemetry.io/collector/consumer v1.60.1-0.20260611154946-8ae0933981eb
	go.opentelemetry.io/collector/exporter v1.60.1-0.20260611154946-8ae0933981eb
	go.opentelemetry.io/collector/exporter/exporterhelper v0.154.1-0.20260611154946-8ae0933981eb
	go.opentelemetry.io/collector/exporter/exportertest v0.154.1-0.20260611154946-8ae0933981eb
	go.opentelemetry.io/collector/otelcol/otelcoltest v0.154.1-0.20260611154946-8ae0933981eb
	go.opentelemetry.io/collector/pdata v1.60.1-0.20260611154946-8ae0933981eb
	go.uber.org/goleak v1.3.0
)

require (
	cloud.google.com/go v0.123.0 // indirect
	cloud.google.com/go/auth v0.20.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	cloud.google.com/go/logging v1.18.0 // indirect
	cloud.google.com/go/longrunning v1.0.0 // indirect
	cloud.google.com/go/monitoring v1.29.0 // indirect
	cloud.google.com/go/trace v1.16.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.21.1 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.13.1 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.12.0 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.6.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v1.33.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.57.0 // indirect
	github.com/alecthomas/units v0.0.0-20240927000941-0f3dac36c52b // indirect
	github.com/aws/aws-sdk-go-v2 v1.41.7 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.32.18 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.19.17 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.23 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.23 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.23 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.24 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.23 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.0.11 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.30.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.36.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.42.1 // indirect
	github.com/aws/smithy-go v1.26.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dennwc/varint v1.0.0 // indirect
	github.com/ebitengine/purego v0.10.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.10.1 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.15 // indirect
	github.com/googleapis/gax-go/v2 v2.22.0 // indirect
	github.com/grafana/regexp v0.0.0-20250905093917-f7b3be9d1853 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.29.0 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.18.6 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.5 // indirect
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
	github.com/prometheus/client_golang/exp v0.0.0-20260518105423-c9d5bc4c50a9 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.68.1 // indirect
	github.com/prometheus/procfs v0.20.1 // indirect
	github.com/prometheus/sigv4 v0.4.1 // indirect
	github.com/shirou/gopsutil/v4 v4.26.5 // indirect
	github.com/spf13/cobra v1.10.2 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/tidwall/gjson v1.19.0 // indirect
	github.com/tidwall/match v1.2.0 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/tinylru v1.2.1 // indirect
	github.com/tidwall/wal v1.2.1 // indirect
	github.com/tklauser/go-sysconf v0.3.16 // indirect
	github.com/tklauser/numcpus v0.11.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/client v1.60.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/component/componentstatus v0.154.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/config/configretry v1.60.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.154.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/confmap/provider/envprovider v1.60.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/confmap/provider/fileprovider v1.60.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/confmap/provider/httpprovider v1.60.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/confmap/provider/yamlprovider v1.60.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/confmap/xconfmap v0.154.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/connector v0.154.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/connector/connectortest v0.154.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/connector/xconnector v0.154.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.154.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/consumer/consumertest v0.154.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.154.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/exporter/xexporter v0.154.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/extension v1.60.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/extension/extensioncapabilities v0.154.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/extension/extensiontest v0.154.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/extension/xextension v0.154.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/featuregate v1.60.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.154.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/internal/fanoutconsumer v0.154.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.154.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/otelcol v0.154.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.154.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/pdata/testdata v0.154.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/pdata/xpdata v0.154.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/pipeline v1.60.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.154.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/processor v1.60.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/processor/processortest v0.154.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/processor/xprocessor v0.154.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/receiver v1.60.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/receiver/receivertest v0.154.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.154.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/service v0.154.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/collector/service/hostcapabilities v0.154.1-0.20260611154946-8ae0933981eb // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.68.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.69.0 // indirect
	go.opentelemetry.io/contrib/otelconf v0.24.0 // indirect
	go.opentelemetry.io/otel v1.44.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc v0.20.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.20.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.44.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.44.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.44.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.44.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.44.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.66.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutlog v0.20.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v1.44.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.44.0 // indirect
	go.opentelemetry.io/otel/log v0.20.0 // indirect
	go.opentelemetry.io/otel/metric v1.44.0 // indirect
	go.opentelemetry.io/otel/sdk v1.44.0 // indirect
	go.opentelemetry.io/otel/sdk/log v0.20.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.44.0 // indirect
	go.opentelemetry.io/otel/trace v1.44.0 // indirect
	go.opentelemetry.io/proto/otlp v1.10.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.28.0 // indirect
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.52.0 // indirect
	golang.org/x/exp v0.0.0-20260527015227-08cc5374adb3 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/oauth2 v0.36.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	gonum.org/v1/gonum v0.17.0 // indirect
	google.golang.org/api v0.280.0 // indirect
	google.golang.org/genproto v0.0.0-20260519071638-aa98bba5eb94 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260526163538-3dc84a4a5aaa // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
	google.golang.org/grpc v1.81.1 // indirect
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
