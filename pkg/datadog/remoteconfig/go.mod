module github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/remoteconfig

// collector-version: minimum

go 1.26.0

require (
	github.com/DataDog/datadog-agent/pkg/config/mock v0.84.0-devel.0.20261030015547-4fcf7d423463
	github.com/DataDog/datadog-agent/pkg/config/model v0.84.0-devel.0.20261030015547-4fcf7d423463
	github.com/DataDog/datadog-agent/pkg/config/remote v0.84.0-devel.0.20261117175822-3905da59c8c4
	github.com/DataDog/datadog-agent/pkg/proto v0.85.0
	github.com/DataDog/datadog-agent/pkg/trace/traceutil v0.84.0-devel.0.20261030015547-4fcf7d423463
	github.com/DataDog/datadog-agent/pkg/version v0.85.0
	github.com/stretchr/testify v1.12.1
	go.opentelemetry.io/collector/component v1.70.0
	go.opentelemetry.io/collector/component/componentstatus v0.164.0
	go.opentelemetry.io/collector/component/componenttest v0.164.0
	go.opentelemetry.io/collector/config/confighttp v0.164.0
	go.opentelemetry.io/collector/config/configopaque v1.70.0
	go.opentelemetry.io/collector/confmap v1.70.0
	go.opentelemetry.io/collector/extension v1.70.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.72.0
	go.opentelemetry.io/otel v1.47.0
	go.opentelemetry.io/otel/metric v1.47.0
	go.opentelemetry.io/otel/sdk/metric v1.47.0
	google.golang.org/grpc v1.85.0
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/DataDog/agent-payload/v5 v5.0.215 // indirect
	github.com/DataDog/datadog-agent/comp/core/delegatedauth v0.84.0-devel.0.20261028224924-1c255663adbd // indirect
	github.com/DataDog/datadog-agent/comp/core/secrets/def v0.84.0-devel.0.20261028224924-1c255663adbd // indirect
	github.com/DataDog/datadog-agent/pkg/config/basic v0.84.0-devel.0.20261028224924-1c255663adbd // indirect
	github.com/DataDog/datadog-agent/pkg/config/create v0.84.0-devel.0.20261028224924-1c255663adbd // indirect
	github.com/DataDog/datadog-agent/pkg/config/env v0.84.0-devel.0.20261028224924-1c255663adbd // indirect
	github.com/DataDog/datadog-agent/pkg/config/helper v0.84.0-devel.0.20261028224924-1c255663adbd // indirect
	github.com/DataDog/datadog-agent/pkg/config/nodetreemodel v0.84.0-devel.0.20261028224924-1c255663adbd // indirect
	github.com/DataDog/datadog-agent/pkg/config/setup v0.84.0-devel.0.20261028224924-1c255663adbd // indirect
	github.com/DataDog/datadog-agent/pkg/config/setup/constants v0.84.0-devel.0.20261028224924-1c255663adbd // indirect
	github.com/DataDog/datadog-agent/pkg/config/setup/generatedsettings v0.0.0-20261028224924-1c255663adbd // indirect
	github.com/DataDog/datadog-agent/pkg/config/structure v0.84.0-devel.0.20261028224924-1c255663adbd // indirect
	github.com/DataDog/datadog-agent/pkg/fips v0.84.0-devel.0.20261028224924-1c255663adbd // indirect
	github.com/DataDog/datadog-agent/pkg/template v0.84.0-devel.0.20261028224924-1c255663adbd // indirect
	github.com/DataDog/datadog-agent/pkg/trace/log v0.77.0-devel.0.20260211235139-a5361978c2b6 // indirect
	github.com/DataDog/datadog-agent/pkg/util/backoff v0.72.0-rc.5 // indirect
	github.com/DataDog/datadog-agent/pkg/util/cache v0.69.4 // indirect
	github.com/DataDog/datadog-agent/pkg/util/defaultpaths v0.84.0-devel.0.20261028224924-1c255663adbd // indirect
	github.com/DataDog/datadog-agent/pkg/util/executable v0.84.0-devel.0.20261028224924-1c255663adbd // indirect
	github.com/DataDog/datadog-agent/pkg/util/filesystem v0.84.0-devel.0.20261028224924-1c255663adbd // indirect
	github.com/DataDog/datadog-agent/pkg/util/http v0.84.0-devel.0.20261028224924-1c255663adbd // indirect
	github.com/DataDog/datadog-agent/pkg/util/log v0.84.0-devel.0.20261028224924-1c255663adbd // indirect
	github.com/DataDog/datadog-agent/pkg/util/pointer v0.84.0-devel.0.20261028224924-1c255663adbd // indirect
	github.com/DataDog/datadog-agent/pkg/util/scrubber v0.84.0-devel.0.20261028224924-1c255663adbd // indirect
	github.com/DataDog/datadog-agent/pkg/util/system v0.84.0-devel.0.20261028224924-1c255663adbd // indirect
	github.com/DataDog/datadog-agent/pkg/util/uuid v0.69.4 // indirect
	github.com/DataDog/datadog-agent/pkg/util/winutil v0.84.0-devel.0.20261028224924-1c255663adbd // indirect
	github.com/DataDog/go-acl v1.0.1 // indirect
	github.com/DataDog/go-tuf v1.1.1-0.5.2 // indirect
	github.com/Masterminds/semver/v3 v3.5.0 // indirect
	github.com/Microsoft/go-winio v0.6.4 // indirect
	github.com/benbjohnson/clock v1.3.5 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/ebitengine/purego v0.11.2 // indirect
	github.com/felixge/httpsnoop v1.1.0 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20251226215517-609e4778396f // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/gofrs/flock v0.13.1 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/go-tpm v0.9.8 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.20.3 // indirect
	github.com/knadh/koanf/maps v0.1.3 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.1 // indirect
	github.com/knadh/koanf/v2 v2.3.8 // indirect
	github.com/lufia/plan9stats v0.0.0-20260802145828-341c2f0c90b5 // indirect
	github.com/mdlayher/socket v0.8.0 // indirect
	github.com/mdlayher/vsock v1.4.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/philhofer/fwd v1.2.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.33 // indirect
	github.com/power-devops/perfstat v0.0.0-20260805114148-88456608a4f6 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/secure-systems-lab/go-securesystemslib v0.11.1 // indirect
	github.com/shirou/gopsutil/v4 v4.26.10 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/tinylib/msgp v1.6.5 // indirect
	github.com/tklauser/go-sysconf v0.4.0 // indirect
	github.com/tklauser/numcpus v0.12.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.etcd.io/bbolt v1.5.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/client v1.70.0 // indirect
	go.opentelemetry.io/collector/config/configauth v1.70.0 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.70.0 // indirect
	go.opentelemetry.io/collector/config/configmiddleware v1.70.0 // indirect
	go.opentelemetry.io/collector/config/confignet v1.70.0 // indirect
	go.opentelemetry.io/collector/config/configoptional v1.70.0 // indirect
	go.opentelemetry.io/collector/config/configtls v1.70.0 // indirect
	go.opentelemetry.io/collector/extension/extensionauth v1.70.0 // indirect
	go.opentelemetry.io/collector/extension/extensionmiddleware v0.164.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.70.0 // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.164.0 // indirect
	go.opentelemetry.io/collector/pdata v1.70.0 // indirect
	go.opentelemetry.io/collector/pipeline v1.70.0 // indirect
	go.opentelemetry.io/otel/log v1.47.0 // indirect
	go.opentelemetry.io/otel/sdk v1.47.0 // indirect
	go.opentelemetry.io/otel/trace v1.47.0 // indirect
	go.uber.org/atomic v1.12.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.28.0 // indirect
	go.yaml.in/yaml/v2 v2.4.5 // indirect
	go.yaml.in/yaml/v3 v3.0.6 // indirect
	golang.org/x/crypto v0.58.0 // indirect
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	golang.org/x/time v0.16.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20261019180522-ce908d29299b // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)
