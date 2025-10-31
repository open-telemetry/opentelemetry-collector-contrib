module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudmonitoringreceiver

go 1.24.0

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden v0.138.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.138.0
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/component v1.44.1-0.20251030084003-6f29b34c24f6
	go.opentelemetry.io/collector/component/componenttest v0.138.1-0.20251030084003-6f29b34c24f6
	go.opentelemetry.io/collector/confmap v1.44.1-0.20251030084003-6f29b34c24f6
	go.opentelemetry.io/collector/consumer v1.44.1-0.20251030084003-6f29b34c24f6
	go.opentelemetry.io/collector/consumer/consumertest v0.138.1-0.20251030084003-6f29b34c24f6
	go.opentelemetry.io/collector/pdata v1.44.1-0.20251030084003-6f29b34c24f6
	go.opentelemetry.io/collector/receiver v1.44.1-0.20251030084003-6f29b34c24f6
	go.opentelemetry.io/collector/receiver/receivertest v0.138.1-0.20251030084003-6f29b34c24f6
	go.opentelemetry.io/collector/scraper/scraperhelper v0.138.1-0.20251030084003-6f29b34c24f6
	go.uber.org/zap v1.27.0
	golang.org/x/oauth2 v0.32.0
	google.golang.org/genproto/googleapis/api v0.0.0-20250804133106-a7a43d27e69b
)

require (
	cloud.google.com/go/auth v0.17.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.6 // indirect
	github.com/googleapis/gax-go/v2 v2.15.0 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.138.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.138.1-0.20251030084003-6f29b34c24f6 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.138.1-0.20251030084003-6f29b34c24f6 // indirect
	go.opentelemetry.io/collector/featuregate v1.44.1-0.20251030084003-6f29b34c24f6 // indirect
	go.opentelemetry.io/collector/pipeline v1.44.1-0.20251030084003-6f29b34c24f6 // indirect
	go.opentelemetry.io/collector/receiver/receiverhelper v0.138.1-0.20251030084003-6f29b34c24f6 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.138.1-0.20251030084003-6f29b34c24f6 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.61.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.43.0 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/time v0.14.0 // indirect
	google.golang.org/genproto v0.0.0-20250603155806-513f23925822 // indirect
)

require (
	cloud.google.com/go/monitoring v1.24.2
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.138.1-0.20251030084003-6f29b34c24f6 // indirect
	go.opentelemetry.io/collector/scraper v0.138.1-0.20251030084003-6f29b34c24f6
	go.opentelemetry.io/otel v1.38.0 // indirect
	go.opentelemetry.io/otel/metric v1.38.0 // indirect
	go.opentelemetry.io/otel/sdk v1.38.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.38.0 // indirect
	go.opentelemetry.io/otel/trace v1.38.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.46.0 // indirect
	golang.org/x/sys v0.37.0 // indirect
	golang.org/x/text v0.30.0 // indirect
	google.golang.org/api v0.254.0
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251022142026-3a174f9686a8 // indirect
	google.golang.org/grpc v1.76.0 // indirect
	google.golang.org/protobuf v1.36.10
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden
