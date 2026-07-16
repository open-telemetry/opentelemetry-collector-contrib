module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter

go 1.25.0

require (
	github.com/aws/aws-sdk-go-v2/config v1.32.29
	github.com/aws/aws-sdk-go-v2/service/servicediscovery v1.41.0
	github.com/aws/smithy-go v1.27.3
	github.com/goccy/go-json v0.10.6
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics v0.156.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal v0.156.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden v0.156.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.156.0
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/component v1.62.1-0.20260709230849-fe96e04e8916
	go.opentelemetry.io/collector/component/componenttest v0.156.1-0.20260709230849-fe96e04e8916
	go.opentelemetry.io/collector/config/configoptional v1.62.1-0.20260709230849-fe96e04e8916
	go.opentelemetry.io/collector/config/configretry v1.62.1-0.20260709230849-fe96e04e8916
	go.opentelemetry.io/collector/confmap v1.62.1-0.20260709230849-fe96e04e8916
	go.opentelemetry.io/collector/consumer v1.62.1-0.20260709230849-fe96e04e8916
	go.opentelemetry.io/collector/consumer/consumererror v0.156.1-0.20260709230849-fe96e04e8916
	go.opentelemetry.io/collector/consumer/consumertest v0.156.1-0.20260709230849-fe96e04e8916
	go.opentelemetry.io/collector/exporter v1.62.1-0.20260709230849-fe96e04e8916
	go.opentelemetry.io/collector/exporter/exporterhelper v0.156.1-0.20260709230849-fe96e04e8916
	go.opentelemetry.io/collector/exporter/exportertest v0.156.1-0.20260709230849-fe96e04e8916
	go.opentelemetry.io/collector/exporter/otlpexporter v0.156.1-0.20260709230849-fe96e04e8916
	go.opentelemetry.io/collector/exporter/xexporter v0.156.1-0.20260709230849-fe96e04e8916
	go.opentelemetry.io/collector/otelcol/otelcoltest v0.156.1-0.20260709230849-fe96e04e8916
	go.opentelemetry.io/collector/pdata v1.62.1-0.20260709230849-fe96e04e8916
	go.opentelemetry.io/otel v1.44.0
	go.opentelemetry.io/otel/metric v1.44.0
	go.opentelemetry.io/otel/sdk/metric v1.44.0
	go.opentelemetry.io/otel/trace v1.44.0
	go.uber.org/goleak v1.3.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.28.0
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/api v0.35.4
	k8s.io/apimachinery v0.35.4
	k8s.io/client-go v0.35.4
	k8s.io/utils v0.0.0-20251002143259-bc988d571ff4
	sigs.k8s.io/controller-runtime v0.23.3
)

require (
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/aws/aws-sdk-go-v2 v1.42.1 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.19.28 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.30 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.30 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.30 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.31 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.30 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.4.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.32.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.37.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.44.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cenkalti/backoff/v7 v7.0.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/ebitengine/purego v0.10.0 // indirect
	github.com/emicklei/go-restful/v3 v3.12.2 // indirect
	github.com/evanphx/json-patch/v5 v5.9.11 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20251226215517-609e4778396f // indirect
	github.com/fsnotify/fsnotify v1.10.1 // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/gnostic-models v0.7.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/go-tpm v0.9.8 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.29.0 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.19.0 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.5 // indirect
	github.com/lufia/plan9stats v0.0.0-20251013123823-9fd1530e3ec3 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.156.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/prometheus/client_golang v1.23.2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.69.0 // indirect
	github.com/prometheus/otlptranslator v1.0.0 // indirect
	github.com/prometheus/procfs v0.20.1 // indirect
	github.com/shirou/gopsutil/v4 v4.26.6 // indirect
	github.com/spf13/cobra v1.10.2 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/tklauser/go-sysconf v0.3.16 // indirect
	github.com/tklauser/numcpus v0.11.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector v0.156.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/client v1.62.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/component/componentstatus v0.156.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/config/configauth v1.62.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.62.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/config/configgrpc v0.156.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/config/configmiddleware v1.62.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/config/confignet v1.62.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.62.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.156.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/config/configtls v1.62.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/confmap/provider/envprovider v1.62.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/confmap/provider/fileprovider v1.62.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/confmap/provider/httpprovider v1.62.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/confmap/provider/yamlprovider v1.62.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/confmap/xconfmap v0.156.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/connector v0.156.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/connector/connectortest v0.156.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/connector/xconnector v0.156.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/consumer/consumererror/xconsumererror v0.156.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.156.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/exporter/exporterhelper/xexporterhelper v0.156.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/extension v1.62.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/extension/extensionauth v1.62.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/extension/extensioncapabilities v0.156.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/extension/extensionmiddleware v0.156.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/extension/extensiontest v0.156.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/extension/xextension v0.156.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/featuregate v1.62.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.156.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/internal/fanoutconsumer v0.156.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.156.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/otelcol v0.156.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.156.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/pdata/testdata v0.156.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/pdata/xpdata v0.156.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/pipeline v1.62.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.156.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/processor v1.62.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/processor/processortest v0.156.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/processor/xprocessor v0.156.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/receiver v1.62.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/receiver/receivertest v0.156.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.156.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/service v0.156.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/collector/service/hostcapabilities v0.156.1-0.20260709230849-fe96e04e8916 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.69.0 // indirect
	go.opentelemetry.io/contrib/otelconf v0.24.0 // indirect
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
	go.opentelemetry.io/otel/sdk v1.44.0 // indirect
	go.opentelemetry.io/otel/sdk/log v0.20.0 // indirect
	go.opentelemetry.io/proto/otlp v1.10.0 // indirect
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.53.0 // indirect
	golang.org/x/exp v0.0.0-20260527015227-08cc5374adb3 // indirect
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/oauth2 v0.36.0 // indirect
	golang.org/x/sys v0.46.0 // indirect
	golang.org/x/term v0.44.0 // indirect
	golang.org/x/text v0.38.0 // indirect
	golang.org/x/time v0.9.0 // indirect
	gonum.org/v1/gonum v0.17.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260526163538-3dc84a4a5aaa // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
	google.golang.org/grpc v1.82.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.13.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/kube-openapi v0.0.0-20250910181357-589584f1c912 // indirect
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.3.2-0.20260122202528-d9cc6641c482 // indirect
	sigs.k8s.io/yaml v1.6.0 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal => ../../pkg/batchpersignal

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics => ../../internal/exp/metrics
