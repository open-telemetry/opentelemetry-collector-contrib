module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver

go 1.22.0

require (
	github.com/DataDog/agent-payload/v5 v5.0.137
	github.com/DataDog/datadog-agent/pkg/obfuscate v0.59.0
	github.com/DataDog/datadog-agent/pkg/proto v0.59.0
	github.com/DataDog/datadog-agent/pkg/trace v0.59.0
	github.com/DataDog/datadog-api-client-go/v2 v2.33.0
	github.com/DataDog/sketches-go v1.4.6
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.115.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics v0.115.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent v0.115.0
	github.com/stretchr/testify v1.10.0
	github.com/tinylib/msgp v1.2.5
	github.com/vmihailenco/msgpack/v5 v5.4.1
	go.opentelemetry.io/collector/component v0.115.1-0.20241206185113-3f3e208e71b8
	go.opentelemetry.io/collector/component/componentstatus v0.115.1-0.20241206185113-3f3e208e71b8
	go.opentelemetry.io/collector/component/componenttest v0.115.1-0.20241206185113-3f3e208e71b8
	go.opentelemetry.io/collector/config/confighttp v0.115.1-0.20241206185113-3f3e208e71b8
	go.opentelemetry.io/collector/confmap v1.21.1-0.20241206185113-3f3e208e71b8
	go.opentelemetry.io/collector/consumer v1.21.1-0.20241206185113-3f3e208e71b8
	go.opentelemetry.io/collector/consumer/consumererror v0.115.1-0.20241206185113-3f3e208e71b8
	go.opentelemetry.io/collector/consumer/consumertest v0.115.1-0.20241206185113-3f3e208e71b8
	go.opentelemetry.io/collector/pdata v1.21.1-0.20241206185113-3f3e208e71b8
	go.opentelemetry.io/collector/receiver v0.115.1-0.20241206185113-3f3e208e71b8
	go.opentelemetry.io/collector/receiver/receivertest v0.115.1-0.20241206185113-3f3e208e71b8
	go.opentelemetry.io/collector/semconv v0.115.1-0.20241206185113-3f3e208e71b8
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.0
	google.golang.org/protobuf v1.35.2
)

require (
	github.com/DataDog/datadog-agent/pkg/remoteconfig/state v0.59.0 // indirect
	github.com/DataDog/datadog-agent/pkg/util/log v0.59.0 // indirect
	github.com/DataDog/datadog-agent/pkg/util/scrubber v0.59.0 // indirect
	github.com/DataDog/datadog-go/v5 v5.5.0 // indirect
	github.com/DataDog/go-sqllexer v0.0.15 // indirect
	github.com/DataDog/go-tuf v1.1.0-0.5.2 // indirect
	github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes v0.20.0 // indirect
	github.com/DataDog/zstd v1.5.5 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cihub/seelog v0.0.0-20170130134532-f561c5e57575 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
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
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.115.0 // indirect
	github.com/outcaste-io/ristretto v0.2.1 // indirect
	github.com/philhofer/fwd v1.1.3-0.20240916144458-20a13a1f6b7c // indirect
	github.com/pierrec/lz4/v4 v4.1.21 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/power-devops/perfstat v0.0.0-20220216144756-c35f1ee13d7c // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/secure-systems-lab/go-securesystemslib v0.7.0 // indirect
	github.com/shirou/gopsutil/v3 v3.24.5 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/vmihailenco/msgpack/v4 v4.3.13 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/collector/client v1.21.1-0.20241206185113-3f3e208e71b8 // indirect
	go.opentelemetry.io/collector/config/configauth v0.115.1-0.20241206185113-3f3e208e71b8 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.21.1-0.20241206185113-3f3e208e71b8 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.21.1-0.20241206185113-3f3e208e71b8 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.115.1-0.20241206185113-3f3e208e71b8 // indirect
	go.opentelemetry.io/collector/config/configtls v1.21.1-0.20241206185113-3f3e208e71b8 // indirect
	go.opentelemetry.io/collector/config/internal v0.115.1-0.20241206185113-3f3e208e71b8 // indirect
	go.opentelemetry.io/collector/consumer/consumerprofiles v0.115.1-0.20241206185113-3f3e208e71b8 // indirect
	go.opentelemetry.io/collector/extension v0.115.1-0.20241206185113-3f3e208e71b8 // indirect
	go.opentelemetry.io/collector/extension/auth v0.115.1-0.20241206185113-3f3e208e71b8 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.115.1-0.20241206185113-3f3e208e71b8 // indirect
	go.opentelemetry.io/collector/pipeline v0.115.1-0.20241206185113-3f3e208e71b8 // indirect
	go.opentelemetry.io/collector/receiver/receiverprofiles v0.115.1-0.20241206185113-3f3e208e71b8 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.56.0 // indirect
	go.opentelemetry.io/otel v1.32.0 // indirect
	go.opentelemetry.io/otel/metric v1.32.0 // indirect
	go.opentelemetry.io/otel/sdk v1.32.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.32.0 // indirect
	go.opentelemetry.io/otel/trace v1.32.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/net v0.31.0 // indirect
	golang.org/x/oauth2 v0.23.0 // indirect
	golang.org/x/sys v0.28.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240903143218-8af14fe29dc1 // indirect
	google.golang.org/grpc v1.68.1 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent => ../../internal/sharedcomponent

retract (
	v0.76.2
	v0.76.1
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics => ../../internal/exp/metrics

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal
