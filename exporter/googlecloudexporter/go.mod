module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudexporter

go 1.26.0

require (
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector v0.60.0
	github.com/stretchr/testify v1.12.1
	go.opentelemetry.io/collector/component v1.66.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/component/componenttest v0.160.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/config/configoptional v1.66.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/confmap v1.66.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/consumer v1.66.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/exporter v1.66.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/exporter/exporterhelper v0.160.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/exporter/exportertest v0.160.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/featuregate v1.66.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/pdata v1.66.1-0.20260903163450-cc4b33fc673f
	go.uber.org/goleak v1.3.0
	google.golang.org/genproto/googleapis/api v0.0.0-20260615183401-62b3387ff324
)

require (
	cloud.google.com/go v0.123.0 // indirect
	cloud.google.com/go/auth v0.20.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	cloud.google.com/go/logging v1.18.0 // indirect
	cloud.google.com/go/longrunning v1.0.0 // indirect
	cloud.google.com/go/monitoring v1.29.0 // indirect
	cloud.google.com/go/trace v1.16.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v1.36.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.60.0 // indirect
	github.com/cenkalti/backoff/v7 v7.0.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.10.1 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.15 // indirect
	github.com/googleapis/gax-go/v2 v2.22.0 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.3 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.1 // indirect
	github.com/knadh/koanf/v2 v2.3.6 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/tidwall/gjson v1.19.0 // indirect
	github.com/tidwall/match v1.2.0 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/tinylru v1.2.1 // indirect
	github.com/tidwall/wal v1.2.1 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/client v1.66.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/config/configretry v1.66.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.160.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/consumer/consumertest v0.160.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.160.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/exporter/xexporter v0.160.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/extension v1.66.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/extension/extensionauth v1.66.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/extension/xextension v0.160.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.160.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.160.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/pdata/xpdata v0.160.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/pipeline v1.66.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.160.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/receiver v1.66.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/receiver/receivertest v0.160.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.160.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.69.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.69.0 // indirect
	go.opentelemetry.io/otel v1.46.0 // indirect
	go.opentelemetry.io/otel/metric v1.46.0 // indirect
	go.opentelemetry.io/otel/sdk v1.46.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.46.0 // indirect
	go.opentelemetry.io/otel/trace v1.46.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.28.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/crypto v0.56.0 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/oauth2 v0.36.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	google.golang.org/api v0.280.0 // indirect
	google.golang.org/genproto v0.0.0-20260519071638-aa98bba5eb94 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260610212136-7ab31c22f7ad // indirect
	google.golang.org/grpc v1.83.2 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
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
