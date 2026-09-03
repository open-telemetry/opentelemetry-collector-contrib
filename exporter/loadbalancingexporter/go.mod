module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter

go 1.26.0

require (
	github.com/aws/aws-sdk-go-v2/config v1.33.1
	github.com/aws/aws-sdk-go-v2/service/servicediscovery v1.46.1
	github.com/aws/smithy-go v1.28.1
	github.com/goccy/go-json v0.10.6
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics v0.160.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal v0.160.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden v0.160.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.160.0
	github.com/stretchr/testify v1.12.1
	go.opentelemetry.io/collector/component v1.66.0
	go.opentelemetry.io/collector/component/componenttest v0.160.0
	go.opentelemetry.io/collector/config/configoptional v1.66.0
	go.opentelemetry.io/collector/config/configretry v1.66.0
	go.opentelemetry.io/collector/confmap v1.66.0
	go.opentelemetry.io/collector/consumer v1.66.0
	go.opentelemetry.io/collector/consumer/consumererror v0.160.0
	go.opentelemetry.io/collector/consumer/consumertest v0.160.0
	go.opentelemetry.io/collector/exporter v1.66.0
	go.opentelemetry.io/collector/exporter/exporterhelper v0.160.0
	go.opentelemetry.io/collector/exporter/exportertest v0.160.0
	go.opentelemetry.io/collector/exporter/otlpexporter v0.160.0
	go.opentelemetry.io/collector/exporter/xexporter v0.160.0
	go.opentelemetry.io/collector/otelcol/otelcoltest v0.160.0
	go.opentelemetry.io/collector/pdata v1.66.0
	go.opentelemetry.io/otel v1.46.0
	go.opentelemetry.io/otel/metric v1.46.0
	go.opentelemetry.io/otel/sdk/metric v1.46.0
	go.opentelemetry.io/otel/trace v1.46.0
	go.uber.org/goleak v1.3.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.28.0
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/api v0.37.0
	k8s.io/apimachinery v0.37.0
	k8s.io/client-go v0.37.0
	sigs.k8s.io/controller-runtime v0.24.1
)

require (
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/aws/aws-sdk-go-v2 v1.45.1 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.20.1 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.19.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.5.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.19 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.14.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.7.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.35.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.40.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.47.1 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cenkalti/backoff/v7 v7.0.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/ebitengine/purego v0.10.2 // indirect
	github.com/emicklei/go-restful/v3 v3.13.0 // indirect
	github.com/evanphx/json-patch/v5 v5.9.11 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20251226215517-609e4778396f // indirect
	github.com/fxamacker/cbor/v2 v2.9.2 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-openapi/jsonpointer v1.0.0 // indirect
	github.com/go-openapi/jsonreference v1.0.0 // indirect
	github.com/go-openapi/swag v0.27.1 // indirect
	github.com/go-openapi/swag/cmdutils v0.27.1 // indirect
	github.com/go-openapi/swag/conv v0.27.1 // indirect
	github.com/go-openapi/swag/fileutils v0.27.1 // indirect
	github.com/go-openapi/swag/jsonutils v0.27.1 // indirect
	github.com/go-openapi/swag/loading v0.27.1 // indirect
	github.com/go-openapi/swag/mangling v0.27.1 // indirect
	github.com/go-openapi/swag/netutils v0.27.1 // indirect
	github.com/go-openapi/swag/pools v0.27.1 // indirect
	github.com/go-openapi/swag/stringutils v0.27.1 // indirect
	github.com/go-openapi/swag/typeutils v0.27.1 // indirect
	github.com/go-openapi/swag/yamlutils v0.27.1 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/gnostic-models v0.7.0 // indirect
	github.com/google/go-tpm v0.9.8 // indirect
	github.com/google/pprof v0.0.0-20260604005048-7023385849c0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.29.0 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.19.2 // indirect
	github.com/knadh/koanf/maps v0.1.3 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.1 // indirect
	github.com/knadh/koanf/v2 v2.3.6 // indirect
	github.com/lufia/plan9stats v0.0.0-20251013123823-9fd1530e3ec3 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.160.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/prometheus/client_golang v1.24.1 // indirect
	github.com/prometheus/client_model v0.6.3 // indirect
	github.com/prometheus/common v0.70.1 // indirect
	github.com/prometheus/otlptranslator v1.0.0 // indirect
	github.com/prometheus/procfs v0.21.1 // indirect
	github.com/shirou/gopsutil/v4 v4.26.8 // indirect
	github.com/spf13/cobra v1.10.2 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/tklauser/go-sysconf v0.3.16 // indirect
	github.com/tklauser/numcpus v0.11.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector v0.160.0 // indirect
	go.opentelemetry.io/collector/client v1.66.0 // indirect
	go.opentelemetry.io/collector/component/componentstatus v0.160.0 // indirect
	go.opentelemetry.io/collector/config/configauth v1.66.0 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.66.0 // indirect
	go.opentelemetry.io/collector/config/configgrpc v1.66.0 // indirect
	go.opentelemetry.io/collector/config/configmiddleware v1.66.0 // indirect
	go.opentelemetry.io/collector/config/confignet v1.66.0 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.66.0 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.160.0 // indirect
	go.opentelemetry.io/collector/config/configtls v1.66.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/envprovider v1.66.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/fileprovider v1.66.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/httpprovider v1.66.0 // indirect
	go.opentelemetry.io/collector/confmap/provider/yamlprovider v1.66.0 // indirect
	go.opentelemetry.io/collector/confmap/xconfmap v0.160.0 // indirect
	go.opentelemetry.io/collector/connector v0.160.0 // indirect
	go.opentelemetry.io/collector/connector/connectortest v0.160.0 // indirect
	go.opentelemetry.io/collector/connector/xconnector v0.160.0 // indirect
	go.opentelemetry.io/collector/consumer/consumererror/xconsumererror v0.160.0 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.160.0 // indirect
	go.opentelemetry.io/collector/exporter/exporterhelper/xexporterhelper v0.160.0 // indirect
	go.opentelemetry.io/collector/extension v1.66.0 // indirect
	go.opentelemetry.io/collector/extension/extensionauth v1.66.0 // indirect
	go.opentelemetry.io/collector/extension/extensioncapabilities v0.160.0 // indirect
	go.opentelemetry.io/collector/extension/extensionmiddleware v0.160.0 // indirect
	go.opentelemetry.io/collector/extension/extensiontest v0.160.0 // indirect
	go.opentelemetry.io/collector/extension/xextension v0.160.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.66.0 // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.160.0 // indirect
	go.opentelemetry.io/collector/internal/fanoutconsumer v0.160.0 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.160.0 // indirect
	go.opentelemetry.io/collector/otelcol v0.160.0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.160.0 // indirect
	go.opentelemetry.io/collector/pdata/testdata v0.160.0 // indirect
	go.opentelemetry.io/collector/pdata/xpdata v0.160.0 // indirect
	go.opentelemetry.io/collector/pipeline v1.66.0 // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.160.0 // indirect
	go.opentelemetry.io/collector/processor v1.66.0 // indirect
	go.opentelemetry.io/collector/processor/processortest v0.160.0 // indirect
	go.opentelemetry.io/collector/processor/xprocessor v0.160.0 // indirect
	go.opentelemetry.io/collector/receiver v1.66.0 // indirect
	go.opentelemetry.io/collector/receiver/receivertest v0.160.0 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.160.0 // indirect
	go.opentelemetry.io/collector/service v0.160.0 // indirect
	go.opentelemetry.io/collector/service/hostcapabilities v0.160.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.70.0 // indirect
	go.opentelemetry.io/contrib/otelconf v0.25.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc v0.21.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.21.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.45.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.45.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.45.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.45.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.45.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.67.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutlog v0.21.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v1.45.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.45.0 // indirect
	go.opentelemetry.io/otel/log v0.22.0 // indirect
	go.opentelemetry.io/otel/sdk v1.46.0 // indirect
	go.opentelemetry.io/otel/sdk/log v0.21.0 // indirect
	go.opentelemetry.io/proto/otlp v1.11.0 // indirect
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/crypto v0.55.0 // indirect
	golang.org/x/exp v0.0.0-20260727155853-b88d891fe743 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/oauth2 v0.36.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/term v0.45.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	gonum.org/v1/gonum v0.17.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260803160001-6ac0973c030d // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260803160001-6ac0973c030d // indirect
	google.golang.org/grpc v1.83.2 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.13.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	k8s.io/klog/v2 v2.140.0 // indirect
	k8s.io/kube-openapi v0.0.0-20260721132016-d427ff9ee9ad // indirect
	k8s.io/utils v0.0.0-20260707023825-cf1189d6abe3 // indirect
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.4.2 // indirect
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
