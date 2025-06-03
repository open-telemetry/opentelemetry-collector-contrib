module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudmonitoringreceiver

go 1.23.0

require (
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v1.33.1-0.20250602081514-8568c97b0d15
	go.opentelemetry.io/collector/component/componenttest v0.127.1-0.20250602081514-8568c97b0d15
	go.opentelemetry.io/collector/confmap v1.33.1-0.20250602081514-8568c97b0d15
	go.opentelemetry.io/collector/consumer v1.33.1-0.20250602081514-8568c97b0d15
	go.opentelemetry.io/collector/consumer/consumertest v0.127.1-0.20250602081514-8568c97b0d15
	go.opentelemetry.io/collector/pdata v1.33.1-0.20250602081514-8568c97b0d15
	go.opentelemetry.io/collector/receiver v1.33.1-0.20250602081514-8568c97b0d15
	go.opentelemetry.io/collector/receiver/receivertest v0.127.1-0.20250602081514-8568c97b0d15
	go.opentelemetry.io/collector/scraper/scraperhelper v0.127.1-0.20250602081514-8568c97b0d15
	go.uber.org/zap v1.27.0
	golang.org/x/oauth2 v0.30.0
	google.golang.org/genproto/googleapis/api v0.0.0-20250505200425-f936aa4a68b2
)

require (
	cloud.google.com/go/auth v0.16.1 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.7.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.6 // indirect
	github.com/googleapis/gax-go/v2 v2.14.2 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.127.1-0.20250602081514-8568c97b0d15 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.127.1-0.20250602081514-8568c97b0d15 // indirect
	go.opentelemetry.io/collector/featuregate v1.33.1-0.20250602081514-8568c97b0d15 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.127.1-0.20250602081514-8568c97b0d15 // indirect
	go.opentelemetry.io/collector/pipeline v0.127.1-0.20250602081514-8568c97b0d15 // indirect
	go.opentelemetry.io/collector/receiver/receiverhelper v0.127.1-0.20250602081514-8568c97b0d15 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.127.1-0.20250602081514-8568c97b0d15 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.11.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.60.0 // indirect
	go.opentelemetry.io/otel/log v0.12.2 // indirect
	golang.org/x/crypto v0.38.0 // indirect
	golang.org/x/sync v0.14.0 // indirect
	golang.org/x/time v0.11.0 // indirect
	google.golang.org/genproto v0.0.0-20250505200425-f936aa4a68b2 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

require (
	cloud.google.com/go/monitoring v1.24.2
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.2.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.127.1-0.20250602081514-8568c97b0d15 // indirect
	go.opentelemetry.io/collector/scraper v0.127.1-0.20250602081514-8568c97b0d15
	go.opentelemetry.io/otel v1.36.0 // indirect
	go.opentelemetry.io/otel/metric v1.36.0 // indirect
	go.opentelemetry.io/otel/sdk v1.36.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.36.0 // indirect
	go.opentelemetry.io/otel/trace v1.36.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.40.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.25.0 // indirect
	google.golang.org/api v0.235.0
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250512202823-5a2f75b736a9 // indirect
	google.golang.org/grpc v1.72.2 // indirect
	google.golang.org/protobuf v1.36.6
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
