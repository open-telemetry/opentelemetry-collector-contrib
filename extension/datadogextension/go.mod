module github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension

go 1.25.0

require (
	github.com/DataDog/datadog-agent/comp/forwarder/defaultforwarder v0.77.0-devel.0.20260213154712-e02b9359151a
	github.com/DataDog/datadog-agent/pkg/config/model v0.77.0-devel.0.20260213154712-e02b9359151a
	github.com/DataDog/datadog-agent/pkg/metrics v0.77.0-devel.0.20260213154712-e02b9359151a
	github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes v0.77.0-devel.0.20260213154712-e02b9359151a
	github.com/DataDog/datadog-agent/pkg/serializer v0.77.0-devel.0.20260213154712-e02b9359151a
	github.com/DataDog/datadog-agent/pkg/tagset v0.77.0-devel.0.20260213154712-e02b9359151a
	github.com/google/uuid v1.6.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog v0.145.0
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/component v1.51.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/component/componentstatus v0.145.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/component/componenttest v0.145.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/config/confighttp v0.145.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/config/confignet v1.51.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/config/configtls v1.51.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/confmap v1.51.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/confmap/provider/fileprovider v1.51.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/extension v1.51.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/extension/extensioncapabilities v0.145.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/extension/extensiontest v0.145.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/otelcol v0.145.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/pdata v1.51.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/service v0.145.1-0.20260212100729-5a059d1d6718
	go.opentelemetry.io/collector/service/hostcapabilities v0.145.1-0.20260212100729-5a059d1d6718
	go.uber.org/zap v1.27.1
)

require (
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	github.com/DataDog/agent-payload/v5 v5.0.179 // indirect
	github.com/DataDog/datadog-agent/comp/core/config v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/comp/core/flare/builder v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/comp/core/flare/types v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/comp/core/log/def v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/comp/core/secrets/def v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/comp/core/secrets/noop-impl v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/comp/core/status v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/comp/core/tagger/def v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/comp/core/tagger/origindetection v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/comp/core/tagger/types v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/comp/core/tagger/utils v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/comp/core/telemetry v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/comp/def v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/comp/forwarder/orchestrator/orchestratorinterface v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/comp/logs/agent/config v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/comp/serializer/metricscompression v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/aggregator/ckey v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/api v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/collector/check/defaults v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/config/basic v0.0.0-20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/config/create v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/config/env v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/config/helper v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/config/mock v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/config/nodetreemodel v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/config/setup v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/config/structure v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/config/teeconfig v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/config/utils v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/config/viperconfig v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/fips v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/logs/types v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/inframetadata v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/metrics v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/orchestrator/model v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/process/util/api v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/proto v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/status/health v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/tagger/types v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/telemetry v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/template v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/trace/log v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/trace/traceutil v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/util/backoff v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/util/buf v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/util/common v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/util/compression v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/util/executable v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/util/filesystem v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/util/fxutil v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/util/hostname/validate v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/util/http v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/util/json v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/util/log v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/util/log/setup v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/util/option v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/util/pointer v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/util/quantile v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/util/scrubber v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/util/sort v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/util/system v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/util/winutil v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-agent/pkg/version v0.77.0-devel.0.20260213154712-e02b9359151a // indirect
	github.com/DataDog/datadog-api-client-go/v2 v2.54.0 // indirect
	github.com/DataDog/gohai v0.0.0-20230524154621-4316413895ee // indirect
	github.com/DataDog/mmh3 v0.0.0-20210722141835-012dc69a9e49 // indirect
	github.com/DataDog/sketches-go v1.4.7 // indirect
	github.com/DataDog/viper v1.15.0 // indirect
	github.com/DataDog/zstd v1.5.7 // indirect
	github.com/DataDog/zstd_0 v0.0.0-20210310093942-586c1286621f // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v1.31.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/aws/aws-sdk-go-v2 v1.41.1 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.32.7 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.19.7 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.17 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.17 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.17 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.289.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.0.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.30.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.35.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.41.6 // indirect
	github.com/aws/smithy-go v1.24.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cihub/seelog v0.0.0-20170130134532-f561c5e57575 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/ebitengine/purego v0.9.1 // indirect
	github.com/emicklei/go-restful/v3 v3.12.2 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20251226215517-609e4778396f // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.21.0 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/gofrs/flock v0.13.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/gnostic-models v0.7.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/go-tpm v0.9.8 // indirect
	github.com/google/pprof v0.0.0-20250607225305-033d6d78b36a // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.27.4 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-version v1.8.0 // indirect
	github.com/hectane/go-acl v0.0.0-20230122075934-ca0b05cb1adb // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.18.4 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.2 // indirect
	github.com/lufia/plan9stats v0.0.0-20251013123823-9fd1530e3ec3 // indirect
	github.com/magiconair/properties v1.8.10 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mdlayher/socket v0.5.1 // indirect
	github.com/mdlayher/vsock v1.2.1 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil v0.145.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.145.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog v0.145.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig v0.145.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders v0.145.0 // indirect
	github.com/openshift/api v0.0.0-20251015095338-264e80a2b6e7 // indirect
	github.com/openshift/client-go v0.0.0-20251015124057-db0dee36e235 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/philhofer/fwd v1.2.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.25 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/prometheus/client_golang v1.23.2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.67.5 // indirect
	github.com/prometheus/otlptranslator v1.0.0 // indirect
	github.com/prometheus/procfs v0.19.2 // indirect
	github.com/richardartoul/molecule v1.0.1-0.20240531184615-7ca0df43c0b3 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/shirou/gopsutil/v3 v3.24.5 // indirect
	github.com/shirou/gopsutil/v4 v4.26.1 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/shoenig/test v1.7.1 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/cobra v1.10.2 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/tinylib/msgp v1.6.3 // indirect
	github.com/tklauser/go-sysconf v0.3.16 // indirect
	github.com/tklauser/numcpus v0.11.0 // indirect
	github.com/twmb/murmur3 v1.1.8 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/client v1.51.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/config/configauth v1.51.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.51.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/config/configmiddleware v1.51.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.51.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/config/configoptional v1.51.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/config/configretry v1.51.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/confmap/xconfmap v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/connector v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/connector/connectortest v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/connector/xconnector v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/consumer v1.51.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/consumer/consumertest v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/exporter v1.51.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/exporter/exporterhelper v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/exporter/exportertest v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/exporter/xexporter v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/extension/extensionauth v1.51.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/extension/extensionmiddleware v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/extension/xextension v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/featuregate v1.51.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/internal/fanoutconsumer v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/pdata/testdata v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/pdata/xpdata v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/pipeline v1.51.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/processor v1.51.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/processor/processortest v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/processor/xprocessor v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/receiver v1.51.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/receiver/receivertest v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.145.1-0.20260212100729-5a059d1d6718 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.64.0 // indirect
	go.opentelemetry.io/contrib/otelconf v0.18.0 // indirect
	go.opentelemetry.io/otel v1.40.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc v0.14.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.14.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.39.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.39.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.39.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.39.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.39.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.60.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutlog v0.14.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v1.39.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.39.0 // indirect
	go.opentelemetry.io/otel/log v0.16.0 // indirect
	go.opentelemetry.io/otel/metric v1.40.0 // indirect
	go.opentelemetry.io/otel/sdk v1.40.0 // indirect
	go.opentelemetry.io/otel/sdk/log v0.14.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.40.0 // indirect
	go.opentelemetry.io/otel/trace v1.40.0 // indirect
	go.opentelemetry.io/proto/otlp v1.9.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/dig v1.19.0 // indirect
	go.uber.org/fx v1.24.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.48.0 // indirect
	golang.org/x/exp v0.0.0-20251209150349-8475f28825e9 // indirect
	golang.org/x/net v0.50.0 // indirect
	golang.org/x/oauth2 v0.35.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/term v0.40.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	golang.org/x/time v0.14.0 // indirect
	gonum.org/v1/gonum v0.17.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20251222181119-0a764e51fe1b // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251222181119-0a764e51fe1b // indirect
	google.golang.org/grpc v1.78.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.13.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/api v0.35.1 // indirect
	k8s.io/apimachinery v0.35.1 // indirect
	k8s.io/client-go v0.35.1 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/kube-openapi v0.0.0-20250910181357-589584f1c912 // indirect
	k8s.io/utils v0.0.0-20251002143259-bc988d571ff4 // indirect
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.3.0 // indirect
	sigs.k8s.io/yaml v1.6.0 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog => ../../pkg/datadog

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders => ../../internal/metadataproviders

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig => ../../internal/k8sconfig

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil => ../../internal/aws/ecsutil

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog => ../../internal/datadog
