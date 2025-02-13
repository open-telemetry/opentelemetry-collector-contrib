module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver

go 1.23.0

require (
	github.com/DataDog/agent-payload/v5 v5.0.144
	github.com/DataDog/datadog-agent/pkg/obfuscate v0.62.2
	github.com/DataDog/datadog-agent/pkg/proto v0.62.2
	github.com/DataDog/datadog-agent/pkg/trace v0.62.2
	github.com/DataDog/datadog-api-client-go/v2 v2.35.0
	github.com/DataDog/sketches-go v1.4.6
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.119.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics v0.119.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent v0.119.0
	github.com/stretchr/testify v1.10.0
	github.com/tinylib/msgp v1.2.5
	github.com/vmihailenco/msgpack/v5 v5.4.1
	go.opentelemetry.io/collector/component v0.119.1-0.20250210123122-44b3eeda354c
	go.opentelemetry.io/collector/component/componentstatus v0.119.1-0.20250210123122-44b3eeda354c
	go.opentelemetry.io/collector/component/componenttest v0.119.1-0.20250210123122-44b3eeda354c
	go.opentelemetry.io/collector/config/confighttp v0.119.1-0.20250210123122-44b3eeda354c
	go.opentelemetry.io/collector/confmap v1.25.1-0.20250210123122-44b3eeda354c
	go.opentelemetry.io/collector/consumer v1.25.1-0.20250210123122-44b3eeda354c
	go.opentelemetry.io/collector/consumer/consumererror v0.119.1-0.20250210123122-44b3eeda354c
	go.opentelemetry.io/collector/consumer/consumertest v0.119.1-0.20250210123122-44b3eeda354c
	go.opentelemetry.io/collector/pdata v1.25.1-0.20250210123122-44b3eeda354c
	go.opentelemetry.io/collector/receiver v0.119.1-0.20250210123122-44b3eeda354c
	go.opentelemetry.io/collector/receiver/receivertest v0.119.1-0.20250210123122-44b3eeda354c
	go.opentelemetry.io/collector/semconv v0.119.1-0.20250210123122-44b3eeda354c
	go.opentelemetry.io/otel/trace v1.34.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.0
	google.golang.org/protobuf v1.36.5
)

require (
	github.com/DataDog/datadog-agent/comp/core/tagger/origindetection v0.62.2 // indirect
	github.com/DataDog/datadog-agent/pkg/remoteconfig/state v0.62.2 // indirect
	github.com/DataDog/datadog-agent/pkg/util/log v0.62.2 // indirect
	github.com/DataDog/datadog-agent/pkg/util/scrubber v0.62.2 // indirect
	github.com/DataDog/datadog-agent/pkg/version v0.62.2 // indirect
	github.com/DataDog/datadog-go/v5 v5.6.0 // indirect
	github.com/DataDog/go-sqllexer v0.0.17 // indirect
	github.com/DataDog/go-tuf v1.1.0-0.5.2 // indirect
	github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes v0.22.0 // indirect
	github.com/DataDog/zstd v1.5.6 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cihub/seelog v0.0.0-20170130134532-f561c5e57575 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/ebitengine/purego v0.8.1 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/knadh/koanf/maps v0.1.1 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/knadh/koanf/v2 v2.1.2 // indirect
	github.com/lufia/plan9stats v0.0.0-20220913051719-115f729f3c8c // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.119.0 // indirect
	github.com/outcaste-io/ristretto v0.2.3 // indirect
	github.com/philhofer/fwd v1.1.3-0.20240916144458-20a13a1f6b7c // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20220216144756-c35f1ee13d7c // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/secure-systems-lab/go-securesystemslib v0.8.0 // indirect
	github.com/shirou/gopsutil/v4 v4.24.11 // indirect
	github.com/tklauser/go-sysconf v0.3.14 // indirect
	github.com/tklauser/numcpus v0.8.0 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/client v1.25.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/config/configauth v0.119.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/config/configcompression v1.25.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/config/configopaque v1.25.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.119.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/config/configtls v1.25.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.119.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/extension v0.119.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/extension/auth v0.119.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.119.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/pipeline v0.119.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.119.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.59.0 // indirect
	go.opentelemetry.io/otel v1.34.0 // indirect
	go.opentelemetry.io/otel/metric v1.34.0 // indirect
	go.opentelemetry.io/otel/sdk v1.34.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.34.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/oauth2 v0.24.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	golang.org/x/time v0.8.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250115164207-1a7da9e5054f // indirect
	google.golang.org/grpc v1.70.0 // indirect
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
