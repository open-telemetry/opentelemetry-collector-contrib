module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver

go 1.24.0

require (
	github.com/DataDog/agent-payload/v5 v5.0.165
	github.com/DataDog/datadog-agent/pkg/obfuscate v0.72.0-devel
	github.com/DataDog/datadog-agent/pkg/proto v0.72.0-devel
	github.com/DataDog/datadog-agent/pkg/trace v0.72.0-devel
	github.com/DataDog/datadog-api-client-go/v2 v2.46.0
	github.com/DataDog/sketches-go v1.4.7
	github.com/hashicorp/golang-lru/v2 v2.0.7
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.136.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog v0.136.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics v0.136.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent v0.136.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog v0.136.0
	github.com/stretchr/testify v1.11.1
	github.com/tinylib/msgp v1.4.0
	github.com/vmihailenco/msgpack/v5 v5.4.1
	go.opentelemetry.io/collector/component v1.42.0
	go.opentelemetry.io/collector/component/componentstatus v0.136.0
	go.opentelemetry.io/collector/component/componenttest v0.136.0
	go.opentelemetry.io/collector/config/confighttp v0.136.0
	go.opentelemetry.io/collector/confmap v1.42.0
	go.opentelemetry.io/collector/consumer v1.42.0
	go.opentelemetry.io/collector/consumer/consumererror v0.136.0
	go.opentelemetry.io/collector/consumer/consumertest v0.136.0
	go.opentelemetry.io/collector/featuregate v1.42.0
	go.opentelemetry.io/collector/pdata v1.42.0
	go.opentelemetry.io/collector/receiver v1.42.0
	go.opentelemetry.io/collector/receiver/receiverhelper v0.136.0
	go.opentelemetry.io/collector/receiver/receivertest v0.136.0
	go.opentelemetry.io/otel v1.38.0
	go.opentelemetry.io/otel/trace v1.38.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.0
	google.golang.org/protobuf v1.36.9
)

require (
	github.com/DataDog/datadog-agent/comp/core/tagger/origindetection v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/metrics v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/remoteconfig/state v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/util/hostname/validate v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/util/log v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/util/quantile v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/util/scrubber v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/version v0.72.0-devel // indirect
	github.com/DataDog/datadog-go/v5 v5.6.0 // indirect
	github.com/DataDog/go-sqllexer v0.1.6 // indirect
	github.com/DataDog/go-tuf v1.1.1-0.5.2 // indirect
	github.com/DataDog/zstd v1.5.6 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cihub/seelog v0.0.0-20170130134532-f561c5e57575 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/ebitengine/purego v0.8.4 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20250903184740-5d135037bd4d // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/go-tpm v0.9.6 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20250317134145-8bc96cf8fc35 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.136.0 // indirect
	github.com/outcaste-io/ristretto v0.2.3 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/philhofer/fwd v1.2.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/secure-systems-lab/go-securesystemslib v0.9.0 // indirect
	github.com/shirou/gopsutil/v4 v4.25.8 // indirect
	github.com/tklauser/go-sysconf v0.3.15 // indirect
	github.com/tklauser/numcpus v0.10.0 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/client v1.42.0 // indirect
	go.opentelemetry.io/collector/config/configauth v0.136.0 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.42.0 // indirect
	go.opentelemetry.io/collector/config/configmiddleware v1.42.0 // indirect
	go.opentelemetry.io/collector/config/confignet v1.42.0 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.42.0 // indirect
	go.opentelemetry.io/collector/config/configoptional v0.136.0 // indirect
	go.opentelemetry.io/collector/config/configretry v1.42.0 // indirect
	go.opentelemetry.io/collector/config/configtls v1.42.0 // indirect
	go.opentelemetry.io/collector/confmap/xconfmap v0.136.0 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.136.0 // indirect
	go.opentelemetry.io/collector/exporter v1.42.0 // indirect
	go.opentelemetry.io/collector/exporter/exporterhelper v0.136.0 // indirect
	go.opentelemetry.io/collector/extension v1.42.0 // indirect
	go.opentelemetry.io/collector/extension/extensionauth v1.42.0 // indirect
	go.opentelemetry.io/collector/extension/extensionmiddleware v0.136.0 // indirect
	go.opentelemetry.io/collector/extension/xextension v0.136.0 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.136.0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.136.0 // indirect
	go.opentelemetry.io/collector/pdata/xpdata v0.136.0 // indirect
	go.opentelemetry.io/collector/pipeline v1.42.0 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.136.0 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.12.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.62.0 // indirect
	go.opentelemetry.io/otel/log v0.14.0 // indirect
	go.opentelemetry.io/otel/metric v1.38.0 // indirect
	go.opentelemetry.io/otel/sdk v1.38.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.38.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.42.0 // indirect
	golang.org/x/net v0.44.0 // indirect
	golang.org/x/oauth2 v0.30.0 // indirect
	golang.org/x/sys v0.36.0 // indirect
	golang.org/x/text v0.29.0 // indirect
	golang.org/x/time v0.12.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250707201910-8d1bb00bc6a7 // indirect
	google.golang.org/grpc v1.75.1 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
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
