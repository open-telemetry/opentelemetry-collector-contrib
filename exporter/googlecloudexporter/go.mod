module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudexporter

go 1.24.0

require (
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector v0.53.0
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/component v1.42.1-0.20250925151503-069408608b28
	go.opentelemetry.io/collector/component/componenttest v0.136.1-0.20250925151503-069408608b28
	go.opentelemetry.io/collector/confmap v1.42.1-0.20250925151503-069408608b28
	go.opentelemetry.io/collector/consumer v1.42.1-0.20250925151503-069408608b28
	go.opentelemetry.io/collector/exporter v1.42.1-0.20250925151503-069408608b28
	go.opentelemetry.io/collector/exporter/exporterhelper v0.136.1-0.20250925151503-069408608b28
	go.opentelemetry.io/collector/exporter/exportertest v0.136.1-0.20250925151503-069408608b28
	go.opentelemetry.io/collector/featuregate v1.42.1-0.20250925151503-069408608b28
	go.opentelemetry.io/collector/pdata v1.42.1-0.20250925151503-069408608b28
	go.uber.org/goleak v1.3.0
	google.golang.org/genproto/googleapis/api v0.0.0-20250707201910-8d1bb00bc6a7
)

require (
	cloud.google.com/go v0.120.0 // indirect
	cloud.google.com/go/auth v0.16.1 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.7.0 // indirect
	cloud.google.com/go/logging v1.13.0 // indirect
	cloud.google.com/go/longrunning v0.6.7 // indirect
	cloud.google.com/go/monitoring v1.24.2 // indirect
	cloud.google.com/go/trace v1.11.6 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v1.29.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.53.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.6 // indirect
	github.com/googleapis/gax-go/v2 v2.14.2 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/tinylru v1.2.1 // indirect
	github.com/tidwall/wal v1.1.8 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/client v1.42.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/config/configoptional v0.136.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/config/configretry v1.42.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/confmap/xconfmap v0.136.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.136.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/consumer/consumertest v0.136.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.136.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/exporter/xexporter v0.136.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/extension v1.42.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/extension/xextension v0.136.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.136.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.136.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/pdata/xpdata v0.136.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/pipeline v1.42.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/receiver v1.42.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/receiver/receivertest v0.136.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.136.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/semconv v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.13.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.60.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.61.0 // indirect
	go.opentelemetry.io/otel v1.38.0 // indirect
	go.opentelemetry.io/otel/log v0.14.0 // indirect
	go.opentelemetry.io/otel/metric v1.38.0 // indirect
	go.opentelemetry.io/otel/sdk v1.38.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.38.0 // indirect
	go.opentelemetry.io/otel/trace v1.38.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.39.0 // indirect
	golang.org/x/net v0.41.0 // indirect
	golang.org/x/oauth2 v0.30.0 // indirect
	golang.org/x/sync v0.15.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/text v0.26.0 // indirect
	golang.org/x/time v0.11.0 // indirect
	google.golang.org/api v0.234.0 // indirect
	google.golang.org/genproto v0.0.0-20250505200425-f936aa4a68b2 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250707201910-8d1bb00bc6a7 // indirect
	google.golang.org/grpc v1.75.1 // indirect
	google.golang.org/protobuf v1.36.9 // indirect
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
