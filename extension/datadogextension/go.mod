module github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension

go 1.23.0

require (
	github.com/DataDog/datadog-agent/pkg/config/model v0.66.1
	github.com/DataDog/datadog-agent/pkg/serializer v0.66.1
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog v0.128.1-0.20250610090210-188191247685
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v1.34.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/component/componentstatus v0.128.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/component/componenttest v0.128.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/config/confighttp v0.128.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/confmap v1.34.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/confmap/provider/fileprovider v1.34.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/extension v1.34.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/extension/extensioncapabilities v0.128.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/extension/extensiontest v0.128.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/otelcol v0.128.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/service v0.128.1-0.20250610090210-188191247685
	go.uber.org/zap v1.27.0
)

require (
	github.com/DataDog/agent-payload/v5 v5.0.147 // indirect
	github.com/DataDog/datadog-agent/comp/core/config v0.66.1 // indirect
	github.com/DataDog/datadog-agent/comp/core/flare/builder v0.66.1 // indirect
	github.com/DataDog/datadog-agent/comp/core/flare/types v0.66.1 // indirect
	github.com/DataDog/datadog-agent/comp/core/log/def v0.66.1 // indirect
	github.com/DataDog/datadog-agent/comp/core/secrets v0.66.1 // indirect
	github.com/DataDog/datadog-agent/comp/core/status v0.66.1 // indirect
	github.com/DataDog/datadog-agent/comp/core/tagger/origindetection v0.66.1 // indirect
	github.com/DataDog/datadog-agent/comp/core/telemetry v0.66.1 // indirect
	github.com/DataDog/datadog-agent/comp/def v0.66.1 // indirect
	github.com/DataDog/datadog-agent/comp/forwarder/defaultforwarder v0.66.1 // indirect
	github.com/DataDog/datadog-agent/comp/forwarder/orchestrator/orchestratorinterface v0.66.1 // indirect
	github.com/DataDog/datadog-agent/comp/serializer/metricscompression v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/aggregator/ckey v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/api v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/collector/check/defaults v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/config/create v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/config/env v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/config/mock v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/config/nodetreemodel v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/config/setup v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/config/structure v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/config/teeconfig v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/config/utils v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/config/viperconfig v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/fips v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/metrics v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/orchestrator/model v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/process/util/api v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/proto v0.68.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/status/health v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/tagger/types v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/tagset v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/telemetry v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/template v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/trace v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/util/backoff v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/util/buf v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/util/common v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/util/compression v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/util/executable v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/util/filesystem v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/util/fxutil v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/util/hostname/validate v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/util/http v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/util/json v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/util/log v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/util/option v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/util/pointer v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/util/scrubber v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/util/sort v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/util/system v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/util/system/socket v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/util/winutil v0.66.1 // indirect
	github.com/DataDog/datadog-agent/pkg/version v0.66.1 // indirect
	github.com/DataDog/mmh3 v0.0.0-20210722141835-012dc69a9e49 // indirect
	github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes v0.28.0 // indirect
	github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/metrics v0.28.0 // indirect
	github.com/DataDog/opentelemetry-mapping-go/pkg/quantile v0.28.0 // indirect
	github.com/DataDog/sketches-go v1.4.7 // indirect
	github.com/DataDog/viper v1.14.0 // indirect
	github.com/DataDog/zstd v1.5.6 // indirect
	github.com/DataDog/zstd_0 v0.0.0-20210310093942-586c1286621f // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v5 v5.0.2 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cihub/seelog v0.0.0-20170130134532-f561c5e57575 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/ebitengine/purego v0.8.4 // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20250323135004-b31fac66206e // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/gofrs/flock v0.12.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/go-tpm v0.9.5 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.26.3 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/hashicorp/hcl v1.0.1-vault-7 // indirect
	github.com/hectane/go-acl v0.0.0-20230122075934-ca0b05cb1adb // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.2.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20240909124753-873cd0166683 // indirect
	github.com/magiconair/properties v1.8.9 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.1-0.20231216201459-8508981c8b6c // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/philhofer/fwd v1.1.3-0.20240916144458-20a13a1f6b7c // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/prometheus/client_golang v1.22.0 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.64.0 // indirect
	github.com/prometheus/procfs v0.16.1 // indirect
	github.com/richardartoul/molecule v1.0.1-0.20240531184615-7ca0df43c0b3 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/shirou/gopsutil/v4 v4.25.5 // indirect
	github.com/spf13/afero v1.12.0 // indirect
	github.com/spf13/cast v1.7.1 // indirect
	github.com/spf13/cobra v1.9.1 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/tinylib/msgp v1.2.5 // indirect
	github.com/tklauser/go-sysconf v0.3.14 // indirect
	github.com/tklauser/numcpus v0.9.0 // indirect
	github.com/twmb/murmur3 v1.1.8 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/client v1.34.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/config/configauth v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.34.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/config/configmiddleware v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/config/confignet v1.34.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.34.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/config/configretry v1.34.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/config/configtls v1.34.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/confmap/xconfmap v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/connector v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/connector/connectortest v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/connector/xconnector v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/consumer v1.34.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/consumer/consumertest v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/exporter v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/exporter/exportertest v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/exporter/xexporter v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/extension/extensionauth v1.34.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/extension/extensionmiddleware v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/extension/xextension v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/featuregate v1.34.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/internal/fanoutconsumer v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/pdata v1.34.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/pdata/testdata v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/pipeline v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/processor v1.34.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/processor/processortest v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/processor/xprocessor v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/receiver v1.34.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/receiver/receivertest v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/semconv v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/service/hostcapabilities v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.11.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.61.0 // indirect
	go.opentelemetry.io/contrib/otelconf v0.16.0 // indirect
	go.opentelemetry.io/contrib/propagators/b3 v1.36.0 // indirect
	go.opentelemetry.io/otel v1.36.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc v0.12.2 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.12.2 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.36.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.36.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.36.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.36.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.36.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.58.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutlog v0.12.2 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v1.36.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.36.0 // indirect
	go.opentelemetry.io/otel/log v0.12.2 // indirect
	go.opentelemetry.io/otel/metric v1.36.0 // indirect
	go.opentelemetry.io/otel/sdk v1.36.0 // indirect
	go.opentelemetry.io/otel/sdk/log v0.12.2 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.36.0 // indirect
	go.opentelemetry.io/otel/trace v1.36.0 // indirect
	go.opentelemetry.io/proto/otlp v1.7.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/dig v1.18.1 // indirect
	go.uber.org/fx v1.23.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.38.0 // indirect
	golang.org/x/exp v0.0.0-20250408133849-7e4ce0ab07d0 // indirect
	golang.org/x/net v0.40.0 // indirect
	golang.org/x/sync v0.14.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.25.0 // indirect
	golang.org/x/time v0.11.0 // indirect
	gonum.org/v1/gonum v0.16.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250528174236-200df99c418a // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250528174236-200df99c418a // indirect
	google.golang.org/grpc v1.72.2 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog => ../../pkg/datadog

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders => ../../internal/metadataproviders

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig => ../../internal/k8sconfig

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil => ../../internal/aws/ecsutil

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog => ../../internal/datadog
