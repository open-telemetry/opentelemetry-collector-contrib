module github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector

go 1.23.0

require (
	github.com/DataDog/datadog-agent/comp/otelcol/otlp/components/metricsclient v0.64.0-rc.13
	github.com/DataDog/datadog-agent/comp/otelcol/otlp/components/statsprocessor v0.64.0-rc.13
	github.com/DataDog/datadog-agent/pkg/obfuscate v0.64.0-rc.13
	github.com/DataDog/datadog-agent/pkg/proto v0.64.0-rc.13
	github.com/DataDog/datadog-agent/pkg/trace v0.64.0-rc.13
	github.com/DataDog/datadog-go/v5 v5.6.0
	github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes v0.26.0
	github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/metrics v0.26.0
	github.com/google/go-cmp v0.7.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter v0.122.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog v0.122.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor v0.122.0
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v1.28.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/component/componenttest v0.122.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/confmap v1.28.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/connector v0.122.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/connector/connectortest v0.122.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/consumer v1.28.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/consumer/consumertest v0.122.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/exporter v0.122.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/exporter/debugexporter v0.122.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/featuregate v1.28.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/otelcol v0.122.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/otelcol/otelcoltest v0.122.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/pdata v1.28.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/pipeline v0.122.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/processor v0.122.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/processor/batchprocessor v0.122.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/receiver v1.28.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/receiver/otlpreceiver v0.122.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/semconv v0.122.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/otel/metric v1.35.0
	go.uber.org/zap v1.27.0
	google.golang.org/protobuf v1.36.5
)

require (
	cloud.google.com/go/compute/metadata v0.6.0 // indirect
	github.com/DataDog/agent-payload/v5 v5.0.145 // indirect
	github.com/DataDog/datadog-agent/comp/core/config v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/comp/core/flare/builder v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/comp/core/flare/types v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/comp/core/hostname/hostnameinterface v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/comp/core/log/def v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/comp/core/secrets v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/comp/core/status v0.63.2 // indirect
	github.com/DataDog/datadog-agent/comp/core/tagger/origindetection v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/comp/core/telemetry v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/comp/def v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/comp/forwarder/defaultforwarder v0.63.2 // indirect
	github.com/DataDog/datadog-agent/comp/forwarder/orchestrator/orchestratorinterface v0.63.2 // indirect
	github.com/DataDog/datadog-agent/comp/logs/agent/config v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/comp/otelcol/logsagentpipeline v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/comp/otelcol/logsagentpipeline/logsagentpipelineimpl v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/comp/otelcol/otlp/components/exporter/logsagentexporter v0.64.0-devel.0.20250218192636-64fdfe7ec366 // indirect
	github.com/DataDog/datadog-agent/comp/otelcol/otlp/components/exporter/serializerexporter v0.65.0-devel.0.20250304124125-23a109221842 // indirect
	github.com/DataDog/datadog-agent/comp/serializer/logscompression v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/comp/serializer/metricscompression v0.63.2 // indirect
	github.com/DataDog/datadog-agent/comp/trace/compression/def v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/comp/trace/compression/impl-gzip v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/aggregator/ckey v0.63.2 // indirect
	github.com/DataDog/datadog-agent/pkg/api v0.63.2 // indirect
	github.com/DataDog/datadog-agent/pkg/collector/check/defaults v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/config/env v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/config/mock v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/config/model v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/config/nodetreemodel v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/config/setup v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/config/structure v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/config/teeconfig v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/config/utils v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/config/viperconfig v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/fips v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/auditor v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/client v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/diagnostic v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/message v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/metrics v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/pipeline v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/processor v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/sds v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/sender v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/sources v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/status/statusinterface v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/status/utils v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/metrics v0.63.2 // indirect
	github.com/DataDog/datadog-agent/pkg/orchestrator/model v0.63.2 // indirect
	github.com/DataDog/datadog-agent/pkg/process/util/api v0.63.2 // indirect
	github.com/DataDog/datadog-agent/pkg/remoteconfig/state v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/serializer v0.63.2 // indirect
	github.com/DataDog/datadog-agent/pkg/status/health v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/tagger/types v0.63.2 // indirect
	github.com/DataDog/datadog-agent/pkg/tagset v0.63.2 // indirect
	github.com/DataDog/datadog-agent/pkg/telemetry v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/util/backoff v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/util/buf v0.63.2 // indirect
	github.com/DataDog/datadog-agent/pkg/util/cgroups v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/util/common v0.63.2 // indirect
	github.com/DataDog/datadog-agent/pkg/util/compression v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/util/executable v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/util/filesystem v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/util/fxutil v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/util/hostname/validate v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/util/http v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/util/json v0.63.2 // indirect
	github.com/DataDog/datadog-agent/pkg/util/log v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/util/option v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/util/pointer v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/util/scrubber v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/util/sort v0.63.2 // indirect
	github.com/DataDog/datadog-agent/pkg/util/startstop v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/util/statstracker v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/util/system v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/util/system/socket v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/util/winutil v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-agent/pkg/version v0.64.0-rc.13 // indirect
	github.com/DataDog/datadog-api-client-go/v2 v2.36.1 // indirect
	github.com/DataDog/dd-sensitive-data-scanner/sds-go/go v0.0.0-20240816154533-f7f9beb53a42 // indirect
	github.com/DataDog/go-sqllexer v0.1.3 // indirect
	github.com/DataDog/go-tuf v1.1.0-0.5.2 // indirect
	github.com/DataDog/gohai v0.0.0-20230524154621-4316413895ee // indirect
	github.com/DataDog/mmh3 v0.0.0-20210722141835-012dc69a9e49 // indirect
	github.com/DataDog/opentelemetry-mapping-go/pkg/inframetadata v0.26.0 // indirect
	github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/logs v0.26.0 // indirect
	github.com/DataDog/opentelemetry-mapping-go/pkg/quantile v0.26.0 // indirect
	github.com/DataDog/sketches-go v1.4.7 // indirect
	github.com/DataDog/viper v1.14.0 // indirect
	github.com/DataDog/zstd v1.5.6 // indirect
	github.com/DataDog/zstd_0 v0.0.0-20210310093942-586c1286621f // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v1.27.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/alecthomas/participle/v2 v2.1.2 // indirect
	github.com/antchfx/xmlquery v1.4.4 // indirect
	github.com/antchfx/xpath v1.3.3 // indirect
	github.com/aws/aws-sdk-go-v2 v1.36.3 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.29.9 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.17.62 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.30 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.34 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.34 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.210.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.12.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.12.15 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.25.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.29.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.33.17 // indirect
	github.com/aws/smithy-go v1.22.2 // indirect
	github.com/benbjohnson/clock v1.3.5 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/briandowns/spinner v1.23.0 // indirect
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.2 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cihub/seelog v0.0.0-20170130134532-f561c5e57575 // indirect
	github.com/containerd/cgroups/v3 v3.0.5 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/ebitengine/purego v0.8.2 // indirect
	github.com/elastic/go-grok v0.3.1 // indirect
	github.com/elastic/lunes v0.1.0 // indirect
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.20.4 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/gofrs/flock v0.12.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/mock v1.7.0-rc.1 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.26.1 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/hashicorp/hcl v1.0.1-vault-5 // indirect
	github.com/hectane/go-acl v0.0.0-20230122075934-ca0b05cb1adb // indirect
	github.com/iancoleman/strcase v0.3.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/karrick/godirwalk v1.17.0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/knadh/koanf/maps v0.1.1 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/knadh/koanf/v2 v2.1.2 // indirect
	github.com/lufia/plan9stats v0.0.0-20240909124753-873cd0166683 // indirect
	github.com/magefile/mage v1.15.0 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.1-0.20231216201459-8508981c8b6c // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/sys/userns v0.1.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/mostynb/go-grpc-compression v1.2.3 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil v0.122.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.122.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.122.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog v0.122.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter v0.122.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig v0.122.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders v0.122.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl v0.122.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry v0.122.0 // indirect
	github.com/opencontainers/runtime-spec v1.2.0 // indirect
	github.com/openshift/api v3.9.0+incompatible // indirect
	github.com/openshift/client-go v0.0.0-20210521082421-73d9475a9142 // indirect
	github.com/outcaste-io/ristretto v0.2.3 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/philhofer/fwd v1.1.3-0.20240916144458-20a13a1f6b7c // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/prometheus/client_golang v1.21.1 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.62.0 // indirect
	github.com/prometheus/procfs v0.16.0 // indirect
	github.com/richardartoul/molecule v1.0.1-0.20240531184615-7ca0df43c0b3 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/secure-systems-lab/go-securesystemslib v0.9.0 // indirect
	github.com/shirou/gopsutil/v3 v3.24.5 // indirect
	github.com/shirou/gopsutil/v4 v4.25.2 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/spf13/afero v1.11.0 // indirect
	github.com/spf13/cast v1.7.1 // indirect
	github.com/spf13/cobra v1.9.1 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/stormcat24/protodep v0.1.8 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/tinylib/msgp v1.2.5 // indirect
	github.com/tklauser/go-sysconf v0.3.14 // indirect
	github.com/tklauser/numcpus v0.9.0 // indirect
	github.com/twmb/murmur3 v1.1.8 // indirect
	github.com/ua-parser/uap-go v0.0.0-20240611065828-3a4781585db6 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/client v1.28.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/component/componentstatus v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/config/configauth v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.28.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/config/configgrpc v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/config/confighttp v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/config/confignet v1.28.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.28.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/config/configretry v1.28.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/config/configtls v1.28.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/confmap/provider/envprovider v1.28.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/confmap/provider/fileprovider v1.28.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/confmap/provider/httpprovider v1.28.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/confmap/provider/yamlprovider v1.28.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/confmap/xconfmap v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/connector/xconnector v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/consumer/consumererror/xconsumererror v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/exporter/exporterhelper/xexporterhelper v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/exporter/exportertest v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/exporter/xexporter v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/extension v1.28.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/extension/extensionauth v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/extension/extensioncapabilities v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/extension/extensiontest v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/extension/xextension v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/internal/fanoutconsumer v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/internal/sharedcomponent v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/pdata/testdata v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/processor/processortest v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/processor/xprocessor v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/receiver/receiverhelper v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/receiver/receivertest v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/service v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/service/hostcapabilities v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.10.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.60.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.60.0 // indirect
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
	go.opentelemetry.io/otel/sdk v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk/log v0.11.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/trace v1.35.0 // indirect
	go.opentelemetry.io/proto/otlp v1.5.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/dig v1.18.0 // indirect
	go.uber.org/fx v1.23.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/exp v0.0.0-20250218142911-aa4b98e5adaa // indirect
	golang.org/x/net v0.37.0 // indirect
	golang.org/x/oauth2 v0.28.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
	golang.org/x/term v0.30.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	golang.org/x/time v0.11.0 // indirect
	gonum.org/v1/gonum v0.15.1 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250218202821-56aae31c358a // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250303144028-a0af3efb3deb // indirect
	google.golang.org/grpc v1.71.0 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.12.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	gopkg.in/zorkian/go-datadog-api.v2 v2.30.0 // indirect
	k8s.io/api v0.32.3 // indirect
	k8s.io/apimachinery v0.32.3 // indirect
	k8s.io/client-go v0.32.3 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/kube-openapi v0.0.0-20241105132330-32ad38e42d3f // indirect
	k8s.io/utils v0.0.0-20241104100929-3ea5e8cea738 // indirect
	sigs.k8s.io/json v0.0.0-20241010143419-9aa6b5e7a4b3 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.5.0 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor => ../../processor/tailsamplingprocessor

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders => ../../internal/metadataproviders

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig => ../../internal/k8sconfig

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker => ../../internal/docker

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest => ../../pkg/xk8stest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry => ../../pkg/resourcetotelemetry

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter => ../../internal/filter

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver => ../../receiver/prometheusreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver => ../../receiver/hostmetricsreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter => ../../exporter/prometheusremotewriteexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver => ../../receiver/filelogreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver => ../../receiver/dockerstatsreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza => ../../pkg/stanza

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor => ../../processor/resourcedetectionprocessor

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl => ../../pkg/ottl

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter => ../../exporter/datadogexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus => ../../pkg/translator/prometheus

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite => ../../pkg/translator/prometheusremotewrite

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor => ../../processor/probabilisticsamplerprocessor

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil => ../../internal/aws/ecsutil

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor => ../../processor/k8sattributesprocessor

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage => ../../extension/storage

replace github.com/openshift/api v3.9.0+incompatible => github.com/openshift/api v0.0.0-20180801171038-322a19404e37

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor => ../../processor/transformprocessor

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling => ../../pkg/sampling

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil => ../../internal/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata => ../../pkg/experimentalmetricmetadata

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog => ../../pkg/datadog

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/core/xidutils => ../../pkg/core/xidutils

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog => ../../internal/datadog
