module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudexporter

go 1.21.0

require (
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector v0.48.1
	github.com/stretchr/testify v1.9.0
	go.opentelemetry.io/collector/component v0.106.1
	go.opentelemetry.io/collector/confmap v0.106.1
	go.opentelemetry.io/collector/exporter v0.106.1
	go.opentelemetry.io/collector/featuregate v1.12.0
	go.opentelemetry.io/collector/pdata v1.12.0
	go.opentelemetry.io/otel/metric v1.28.0
	go.opentelemetry.io/otel/trace v1.28.0
	go.uber.org/goleak v1.3.0
)

require (
	cloud.google.com/go v0.115.0 // indirect
	cloud.google.com/go/auth v0.7.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.3 // indirect
	cloud.google.com/go/compute/metadata v0.5.0 // indirect
	cloud.google.com/go/logging v1.10.0 // indirect
	cloud.google.com/go/longrunning v0.5.9 // indirect
	cloud.google.com/go/monitoring v1.20.2 // indirect
	cloud.google.com/go/trace v1.10.10 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v1.24.1 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.48.1 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.0.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/s2a-go v0.1.7 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.2 // indirect
	github.com/googleapis/gax-go/v2 v2.12.5 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.1 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/knadh/koanf/v2 v2.1.1 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_golang v1.19.1 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.55.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/tidwall/gjson v1.10.2 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	github.com/tidwall/tinylru v1.1.0 // indirect
	github.com/tidwall/wal v1.1.7 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/collector v0.106.1 // indirect
	go.opentelemetry.io/collector/config/configretry v1.12.0 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.106.1 // indirect
	go.opentelemetry.io/collector/consumer v0.106.1 // indirect
	go.opentelemetry.io/collector/consumer/consumerprofiles v0.106.1 // indirect
	go.opentelemetry.io/collector/consumer/consumertest v0.106.1 // indirect
	go.opentelemetry.io/collector/extension v0.106.1 // indirect
	go.opentelemetry.io/collector/internal/globalgates v0.106.1 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.106.1 // indirect
	go.opentelemetry.io/collector/receiver v0.106.1 // indirect
	go.opentelemetry.io/collector/semconv v0.106.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.52.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.53.0 // indirect
	go.opentelemetry.io/otel v1.28.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.50.0 // indirect
	go.opentelemetry.io/otel/sdk v1.28.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.28.0 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/crypto v0.25.0 // indirect
	golang.org/x/net v0.27.0 // indirect
	golang.org/x/oauth2 v0.21.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
	golang.org/x/sys v0.22.0 // indirect
	golang.org/x/text v0.16.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	google.golang.org/api v0.188.0 // indirect
	google.golang.org/genproto v0.0.0-20240708141625-4ad9e859172b // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240709173604-40e1e62336c5 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240709173604-40e1e62336c5 // indirect
	google.golang.org/grpc v1.65.0 // indirect
	google.golang.org/protobuf v1.34.2 // indirect
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
