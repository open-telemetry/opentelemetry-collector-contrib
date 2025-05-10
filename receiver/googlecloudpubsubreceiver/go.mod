module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver

go 1.23.0

require (
	cloud.google.com/go/logging v1.13.0
	cloud.google.com/go/pubsub v1.49.0
	github.com/google/go-cmp v0.7.0
	github.com/googleapis/gax-go/v2 v2.14.1
	github.com/iancoleman/strcase v0.3.0
	github.com/json-iterator/go v1.1.12
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding v0.125.0
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v1.31.1-0.20250509190408-4ca0f1829e0a
	go.opentelemetry.io/collector/component/componenttest v0.125.1-0.20250509190408-4ca0f1829e0a
	go.opentelemetry.io/collector/confmap v1.31.1-0.20250509190408-4ca0f1829e0a
	go.opentelemetry.io/collector/confmap/xconfmap v0.125.1-0.20250509190408-4ca0f1829e0a
	go.opentelemetry.io/collector/consumer v1.31.1-0.20250509190408-4ca0f1829e0a
	go.opentelemetry.io/collector/consumer/consumertest v0.125.1-0.20250509190408-4ca0f1829e0a
	go.opentelemetry.io/collector/exporter v0.125.1-0.20250509190408-4ca0f1829e0a
	go.opentelemetry.io/collector/pdata v1.31.1-0.20250509190408-4ca0f1829e0a
	go.opentelemetry.io/collector/receiver v1.31.1-0.20250509190408-4ca0f1829e0a
	go.opentelemetry.io/collector/receiver/receiverhelper v0.125.1-0.20250509190408-4ca0f1829e0a
	go.opentelemetry.io/collector/receiver/receivertest v0.125.1-0.20250509190408-4ca0f1829e0a
	go.opentelemetry.io/otel v1.35.0
	go.opentelemetry.io/otel/metric v1.35.0
	go.opentelemetry.io/otel/sdk/metric v1.35.0
	go.opentelemetry.io/otel/trace v1.35.0
	go.uber.org/goleak v1.3.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.0
	google.golang.org/api v0.232.0
	google.golang.org/genproto v0.0.0-20250303144028-a0af3efb3deb
	google.golang.org/genproto/googleapis/api v0.0.0-20250313205543-e70fdf4c4cb4
	google.golang.org/grpc v1.72.0
	google.golang.org/protobuf v1.36.6
)

require (
	cloud.google.com/go v0.120.0 // indirect
	cloud.google.com/go/auth v0.16.1 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.6.0 // indirect
	cloud.google.com/go/iam v1.4.2 // indirect
	cloud.google.com/go/longrunning v0.6.5 // indirect
	github.com/cenkalti/backoff/v5 v5.0.2 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.6 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.2.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	go.einride.tech/aip v0.68.1 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/config/configretry v1.31.1-0.20250509190408-4ca0f1829e0a // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.125.1-0.20250509190408-4ca0f1829e0a // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.125.1-0.20250509190408-4ca0f1829e0a // indirect
	go.opentelemetry.io/collector/extension v1.31.1-0.20250509190408-4ca0f1829e0a // indirect
	go.opentelemetry.io/collector/extension/xextension v0.125.1-0.20250509190408-4ca0f1829e0a // indirect
	go.opentelemetry.io/collector/featuregate v1.31.1-0.20250509190408-4ca0f1829e0a // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.125.1-0.20250509190408-4ca0f1829e0a // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.125.1-0.20250509190408-4ca0f1829e0a // indirect
	go.opentelemetry.io/collector/pipeline v0.125.1-0.20250509190408-4ca0f1829e0a // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.125.1-0.20250509190408-4ca0f1829e0a // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.10.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.60.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.60.0 // indirect
	go.opentelemetry.io/otel/log v0.11.0 // indirect
	go.opentelemetry.io/otel/sdk v1.35.0 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/net v0.39.0 // indirect
	golang.org/x/oauth2 v0.30.0 // indirect
	golang.org/x/sync v0.14.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	golang.org/x/time v0.11.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250428153025-10db94c68c34 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding => ../../extension/encoding
