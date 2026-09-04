module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver

go 1.26.0

require (
	github.com/DataDog/agent-payload/v5 v5.0.209
	github.com/DataDog/datadog-agent/pkg/obfuscate v0.82.3
	github.com/DataDog/datadog-agent/pkg/proto v0.82.3
	github.com/DataDog/datadog-agent/pkg/trace v0.82.3
	github.com/DataDog/datadog-agent/pkg/trace/stats v0.82.3
	github.com/DataDog/datadog-agent/pkg/trace/traceutil v0.82.3
	github.com/DataDog/datadog-api-client-go/v2 v2.64.0
	github.com/DataDog/sketches-go v1.4.8
	github.com/hashicorp/golang-lru/v2 v2.0.7
	github.com/klauspost/compress v1.19.2
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.160.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog v0.160.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics v0.160.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent v0.160.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog v0.160.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden v0.160.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.160.0
	github.com/stretchr/testify v1.12.1
	github.com/tinylib/msgp v1.6.4
	github.com/vmihailenco/msgpack/v5 v5.4.1
	go.opentelemetry.io/collector/component v1.66.0
	go.opentelemetry.io/collector/component/componentstatus v0.160.0
	go.opentelemetry.io/collector/component/componenttest v0.160.0
	go.opentelemetry.io/collector/config/confighttp v0.160.0
	go.opentelemetry.io/collector/config/confignet v1.66.0
	go.opentelemetry.io/collector/confmap v1.66.0
	go.opentelemetry.io/collector/consumer v1.66.0
	go.opentelemetry.io/collector/consumer/consumererror v0.160.0
	go.opentelemetry.io/collector/consumer/consumertest v0.160.0
	go.opentelemetry.io/collector/featuregate v1.66.0
	go.opentelemetry.io/collector/pdata v1.66.0
	go.opentelemetry.io/collector/receiver v1.66.0
	go.opentelemetry.io/collector/receiver/receiverhelper v0.160.0
	go.opentelemetry.io/collector/receiver/receivertest v0.160.0
	go.opentelemetry.io/otel v1.46.0
	go.opentelemetry.io/otel/trace v1.46.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.28.0
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/DataDog/datadog-agent/comp/core/tagger/origindetection v0.82.3 // indirect
	github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes v0.83.0-devel.0.20260714134811-fee4bbf7ff73 // indirect
	github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/metrics v0.83.0-devel.0.20260714134811-fee4bbf7ff73 // indirect
	github.com/DataDog/datadog-agent/pkg/remoteconfig/state v0.82.3 // indirect
	github.com/DataDog/datadog-agent/pkg/template v0.82.3 // indirect
	github.com/DataDog/datadog-agent/pkg/trace/log v0.82.3 // indirect
	github.com/DataDog/datadog-agent/pkg/util/hostname/validate v0.82.0 // indirect
	github.com/DataDog/datadog-agent/pkg/util/log v0.82.3 // indirect
	github.com/DataDog/datadog-agent/pkg/util/quantile v0.82.0 // indirect
	github.com/DataDog/datadog-agent/pkg/util/scrubber v0.82.3 // indirect
	github.com/DataDog/datadog-agent/pkg/version v0.82.3 // indirect
	github.com/DataDog/datadog-go/v5 v5.9.1 // indirect
	github.com/DataDog/go-sqllexer v0.2.3 // indirect
	github.com/DataDog/go-tuf v1.1.1-0.5.2 // indirect
	github.com/DataDog/zstd v1.5.8-0.20260421145859-31a7e515a571 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/cenkalti/backoff/v7 v7.0.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/ebitengine/purego v0.10.2 // indirect
	github.com/felixge/httpsnoop v1.1.0 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20251226215517-609e4778396f // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/go-tpm v0.9.8 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.3 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.1 // indirect
	github.com/knadh/koanf/v2 v2.3.6 // indirect
	github.com/lufia/plan9stats v0.0.0-20260330125221-c963978e514e // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.160.0 // indirect
	github.com/outcaste-io/ristretto v0.2.3 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/philhofer/fwd v1.2.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.29 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/secure-systems-lab/go-securesystemslib v0.11.0 // indirect
	github.com/shirou/gopsutil/v4 v4.26.8 // indirect
	github.com/tklauser/go-sysconf v0.4.0 // indirect
	github.com/tklauser/numcpus v0.12.0 // indirect
	github.com/twmb/murmur3 v1.1.8 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/client v1.66.0 // indirect
	go.opentelemetry.io/collector/config/configauth v1.66.0 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.66.0 // indirect
	go.opentelemetry.io/collector/config/configmiddleware v1.66.0 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.66.0 // indirect
	go.opentelemetry.io/collector/config/configoptional v1.66.0 // indirect
	go.opentelemetry.io/collector/config/configretry v1.66.0 // indirect
	go.opentelemetry.io/collector/config/configtls v1.66.0 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.160.0 // indirect
	go.opentelemetry.io/collector/exporter v1.66.0 // indirect
	go.opentelemetry.io/collector/exporter/exporterhelper v0.160.0 // indirect
	go.opentelemetry.io/collector/extension v1.66.0 // indirect
	go.opentelemetry.io/collector/extension/extensionauth v1.66.0 // indirect
	go.opentelemetry.io/collector/extension/extensionmiddleware v0.160.0 // indirect
	go.opentelemetry.io/collector/extension/xextension v0.160.0 // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.160.0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.160.0 // indirect
	go.opentelemetry.io/collector/pdata/xpdata v0.160.0 // indirect
	go.opentelemetry.io/collector/pipeline v1.66.0 // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.160.0 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.160.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.70.0 // indirect
	go.opentelemetry.io/otel/metric v1.46.0 // indirect
	go.opentelemetry.io/otel/sdk v1.46.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.46.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/mock v0.6.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/crypto v0.55.0 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/oauth2 v0.36.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260618152121-87f3d3e198d3 // indirect
	google.golang.org/grpc v1.83.2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent => ../../internal/sharedcomponent

retract (
	v0.76.2
	v0.76.1
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics => ../../internal/exp/metrics

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog => ../../internal/datadog

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog => ../../pkg/datadog

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders => ../../internal/metadataproviders

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig => ../../internal/k8sconfig

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil => ../../internal/aws/ecsutil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling => ../../pkg/sampling
