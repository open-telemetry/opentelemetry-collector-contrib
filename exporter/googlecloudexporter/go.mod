module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudexporter

go 1.22.7

require (
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector v0.50.0
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v0.119.0
	go.opentelemetry.io/collector/component/componenttest v0.119.0
	go.opentelemetry.io/collector/confmap v1.25.0
	go.opentelemetry.io/collector/consumer v1.25.0
	go.opentelemetry.io/collector/exporter v0.119.0
	go.opentelemetry.io/collector/exporter/exportertest v0.119.0
	go.opentelemetry.io/collector/featuregate v1.25.0
	go.opentelemetry.io/collector/pdata v1.25.0
	go.uber.org/goleak v1.3.0
)

require (
	cloud.google.com/go v0.118.0 // indirect
	cloud.google.com/go/auth v0.14.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.7 // indirect
	cloud.google.com/go/compute/metadata v0.6.0 // indirect
	cloud.google.com/go/logging v1.13.0 // indirect
	cloud.google.com/go/longrunning v0.6.4 // indirect
	cloud.google.com/go/monitoring v1.22.1 // indirect
	cloud.google.com/go/trace v1.11.3 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v1.26.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.50.0 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.4 // indirect
	github.com/googleapis/gax-go/v2 v2.14.1 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.1 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/knadh/koanf/v2 v2.1.2 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/tinylru v1.2.1 // indirect
	github.com/tidwall/wal v1.1.8 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/config/configretry v1.25.0 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.119.0 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.119.0 // indirect
	go.opentelemetry.io/collector/consumer/consumertest v0.119.0 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.119.0 // indirect
	go.opentelemetry.io/collector/exporter/xexporter v0.119.0 // indirect
	go.opentelemetry.io/collector/extension v0.119.0 // indirect
	go.opentelemetry.io/collector/extension/xextension v0.119.0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.119.0 // indirect
	go.opentelemetry.io/collector/pipeline v0.119.0 // indirect
	go.opentelemetry.io/collector/receiver v0.119.0 // indirect
	go.opentelemetry.io/collector/receiver/receivertest v0.119.0 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.119.0 // indirect
	go.opentelemetry.io/collector/semconv v0.119.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.56.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.56.0 // indirect
	go.opentelemetry.io/otel v1.34.0 // indirect
	go.opentelemetry.io/otel/metric v1.34.0 // indirect
	go.opentelemetry.io/otel/sdk v1.34.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.34.0 // indirect
	go.opentelemetry.io/otel/trace v1.34.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/crypto v0.32.0 // indirect
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/oauth2 v0.25.0 // indirect
	golang.org/x/sync v0.10.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	golang.org/x/time v0.9.0 // indirect
	google.golang.org/api v0.216.0 // indirect
	google.golang.org/genproto v0.0.0-20250106144421-5f5ef82da422 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250106144421-5f5ef82da422 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250106144421-5f5ef82da422 // indirect
	google.golang.org/grpc v1.70.0 // indirect
	google.golang.org/protobuf v1.36.4 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

retract (
	v0.88.0
	v0.87.0
	v0.86.0
	v0.85.0
	v0.84.0
	v0.76.2
	v0.76.1
	v0.65.0
)
