module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver

go 1.25.0

require (
	cloud.google.com/go/pubsub/v2 v2.4.0
	github.com/googleapis/gax-go/v2 v2.18.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding v0.147.0
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/component v1.53.1-0.20260309153054-85fc1918516c
	go.opentelemetry.io/collector/component/componenttest v0.147.1-0.20260309153054-85fc1918516c
	go.opentelemetry.io/collector/confmap v1.53.1-0.20260309153054-85fc1918516c
	go.opentelemetry.io/collector/confmap/xconfmap v0.147.1-0.20260309153054-85fc1918516c
	go.opentelemetry.io/collector/consumer v1.53.1-0.20260309153054-85fc1918516c
	go.opentelemetry.io/collector/consumer/consumertest v0.147.1-0.20260309153054-85fc1918516c
	go.opentelemetry.io/collector/exporter/exporterhelper v0.147.1-0.20260309153054-85fc1918516c
	go.opentelemetry.io/collector/pdata v1.53.1-0.20260309153054-85fc1918516c
	go.opentelemetry.io/collector/receiver v1.53.1-0.20260309153054-85fc1918516c
	go.opentelemetry.io/collector/receiver/receiverhelper v0.147.1-0.20260309153054-85fc1918516c
	go.opentelemetry.io/collector/receiver/receivertest v0.147.1-0.20260309153054-85fc1918516c
	go.opentelemetry.io/otel v1.42.0
	go.opentelemetry.io/otel/metric v1.42.0
	go.opentelemetry.io/otel/sdk/metric v1.42.0
	go.opentelemetry.io/otel/trace v1.42.0
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.1
	google.golang.org/api v0.267.0
	google.golang.org/genproto v0.0.0-20260217215200-42d3e9bedb6d
	google.golang.org/grpc v1.79.2
)

require (
	cloud.google.com/go v0.123.0 // indirect
	cloud.google.com/go/auth v0.18.1 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	cloud.google.com/go/iam v1.5.3 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.11 // indirect
	github.com/hashicorp/go-version v1.8.0 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.2 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	go.einride.tech/aip v0.79.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/client v1.53.1-0.20260309153054-85fc1918516c // indirect
	go.opentelemetry.io/collector/config/configoptional v1.53.1-0.20260309153054-85fc1918516c // indirect
	go.opentelemetry.io/collector/config/configretry v1.53.1-0.20260309153054-85fc1918516c // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.147.1-0.20260309153054-85fc1918516c // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.147.1-0.20260309153054-85fc1918516c // indirect
	go.opentelemetry.io/collector/exporter v1.53.1-0.20260309153054-85fc1918516c // indirect
	go.opentelemetry.io/collector/extension v1.53.1-0.20260309153054-85fc1918516c // indirect
	go.opentelemetry.io/collector/extension/xextension v0.147.1-0.20260309153054-85fc1918516c // indirect
	go.opentelemetry.io/collector/featuregate v1.53.1-0.20260309153054-85fc1918516c // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.147.1-0.20260309153054-85fc1918516c // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.147.1-0.20260309153054-85fc1918516c // indirect
	go.opentelemetry.io/collector/pdata/xpdata v0.147.1-0.20260309153054-85fc1918516c // indirect
	go.opentelemetry.io/collector/pipeline v1.53.1-0.20260309153054-85fc1918516c // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.147.1-0.20260309153054-85fc1918516c // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.147.1-0.20260309153054-85fc1918516c // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.61.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.61.0 // indirect
	go.opentelemetry.io/otel/sdk v1.42.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.48.0 // indirect
	golang.org/x/net v0.51.0 // indirect
	golang.org/x/oauth2 v0.35.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	golang.org/x/time v0.14.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260217215200-42d3e9bedb6d // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260217215200-42d3e9bedb6d // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding => ../../extension/encoding
