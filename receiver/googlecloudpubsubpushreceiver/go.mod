module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubpushreceiver

go 1.24.0

require (
	cloud.google.com/go/storage v1.58.0
	github.com/goccy/go-json v0.10.5
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding v0.142.0
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/client v1.48.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/component v1.48.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/component/componentstatus v0.142.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/component/componenttest v0.142.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/config/confighttp v0.142.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/config/configoptional v1.48.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/config/configtls v1.48.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/confmap v1.48.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/confmap/xconfmap v0.142.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/consumer v1.48.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/consumer/consumererror v0.142.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/consumer/consumertest v0.142.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/extension v1.48.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/pdata v1.48.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/receiver v1.48.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/receiver/receivertest v0.142.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/otel v1.39.0
	go.opentelemetry.io/otel/metric v1.39.0
	go.opentelemetry.io/otel/sdk/metric v1.39.0
	go.opentelemetry.io/otel/trace v1.39.0
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.1
)

require (
	cel.dev/expr v0.24.0 // indirect
	cloud.google.com/go v0.123.0 // indirect
	cloud.google.com/go/auth v0.17.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	cloud.google.com/go/iam v1.5.3 // indirect
	cloud.google.com/go/monitoring v1.24.2 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v1.30.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric v0.54.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.54.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cncf/xds/go v0.0.0-20251022180443-0feb69152e9f // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/envoyproxy/go-control-plane/envoy v1.35.0 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.2.1 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20250903184740-5d135037bd4d // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-jose/go-jose/v4 v4.1.3 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/go-tpm v0.9.8 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.7 // indirect
	github.com/googleapis/gax-go/v2 v2.15.0 // indirect
	github.com/hashicorp/go-version v1.8.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.18.2 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/pierrec/lz4/v4 v4.1.23 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/spiffe/go-spiffe/v2 v2.6.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/config/configauth v1.48.1-0.20251231114627-ad49dc64648d // indirect
	go.opentelemetry.io/collector/config/configcompression v1.48.1-0.20251231114627-ad49dc64648d // indirect
	go.opentelemetry.io/collector/config/configmiddleware v1.48.1-0.20251231114627-ad49dc64648d // indirect
	go.opentelemetry.io/collector/config/configopaque v1.48.1-0.20251231114627-ad49dc64648d // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.142.1-0.20251231114627-ad49dc64648d // indirect
	go.opentelemetry.io/collector/extension/extensionauth v1.48.1-0.20251231114627-ad49dc64648d // indirect
	go.opentelemetry.io/collector/extension/extensionmiddleware v0.142.1-0.20251231114627-ad49dc64648d // indirect
	go.opentelemetry.io/collector/featuregate v1.48.1-0.20251231114627-ad49dc64648d // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.142.1-0.20251231114627-ad49dc64648d // indirect
	go.opentelemetry.io/collector/pipeline v1.48.1-0.20251231114627-ad49dc64648d // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.142.1-0.20251231114627-ad49dc64648d // indirect
	go.opentelemetry.io/contrib/detectors/gcp v1.38.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.63.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.63.0 // indirect
	go.opentelemetry.io/otel/sdk v1.39.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.46.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/oauth2 v0.33.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	golang.org/x/time v0.14.0 // indirect
	google.golang.org/api v0.256.0 // indirect
	google.golang.org/genproto v0.0.0-20250922171735-9219d122eba9 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20251111163417-95abcf5c77ba // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251111163417-95abcf5c77ba // indirect
	google.golang.org/grpc v1.78.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding => ../../extension/encoding
