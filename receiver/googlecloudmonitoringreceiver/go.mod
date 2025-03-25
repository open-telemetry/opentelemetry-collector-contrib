module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudmonitoringreceiver

go 1.23.0

require (
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v1.28.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/component/componenttest v0.122.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/confmap v1.28.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/consumer v1.28.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/consumer/consumertest v0.122.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/pdata v1.28.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/receiver v1.28.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/receiver/receivertest v0.122.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/collector/scraper/scraperhelper v0.122.2-0.20250319144947-41a9ea7f7402
	go.uber.org/zap v1.27.0
	golang.org/x/oauth2 v0.28.0
	google.golang.org/genproto/googleapis/api v0.0.0-20250303144028-a0af3efb3deb
)

require (
	cloud.google.com/go/auth v0.15.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.7 // indirect
	cloud.google.com/go/compute/metadata v0.6.0 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.6 // indirect
	github.com/googleapis/gax-go/v2 v2.14.1 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/featuregate v1.28.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/pipeline v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/receiver/receiverhelper v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.59.0 // indirect
	golang.org/x/crypto v0.36.0 // indirect
	golang.org/x/sync v0.12.0 // indirect
	golang.org/x/time v0.11.0 // indirect
	google.golang.org/genproto v0.0.0-20250303144028-a0af3efb3deb // indirect
)

require (
	cloud.google.com/go/monitoring v1.24.1
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.1 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/knadh/koanf/v2 v2.1.2 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.122.2-0.20250319144947-41a9ea7f7402 // indirect
	go.opentelemetry.io/collector/scraper v0.122.2-0.20250319144947-41a9ea7f7402
	go.opentelemetry.io/otel v1.35.0 // indirect
	go.opentelemetry.io/otel/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/trace v1.35.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.37.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	google.golang.org/api v0.227.0
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250313205543-e70fdf4c4cb4 // indirect
	google.golang.org/grpc v1.71.0 // indirect
	google.golang.org/protobuf v1.36.6
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
