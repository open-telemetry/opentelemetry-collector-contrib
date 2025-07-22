module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver

go 1.23.0

require (
	cloud.google.com/go/spanner v1.83.0
	github.com/jellydator/ttlcache/v3 v3.4.0
	github.com/mitchellh/hashstructure/v2 v2.0.2
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v1.36.1-0.20250715222903-0a7598ec1e19
	go.opentelemetry.io/collector/component/componenttest v0.130.1-0.20250715222903-0a7598ec1e19
	go.opentelemetry.io/collector/confmap v1.36.1-0.20250715222903-0a7598ec1e19
	go.opentelemetry.io/collector/confmap/xconfmap v0.130.1-0.20250715222903-0a7598ec1e19
	go.opentelemetry.io/collector/consumer v1.36.1-0.20250715222903-0a7598ec1e19
	go.opentelemetry.io/collector/consumer/consumertest v0.130.1-0.20250715222903-0a7598ec1e19
	go.opentelemetry.io/collector/pdata v1.36.1-0.20250715222903-0a7598ec1e19
	go.opentelemetry.io/collector/receiver v1.36.1-0.20250715222903-0a7598ec1e19
	go.opentelemetry.io/collector/receiver/receivertest v0.130.1-0.20250715222903-0a7598ec1e19
	go.opentelemetry.io/collector/scraper v0.130.1-0.20250715222903-0a7598ec1e19
	go.opentelemetry.io/collector/scraper/scraperhelper v0.130.1-0.20250715222903-0a7598ec1e19
	go.uber.org/goleak v1.3.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.0
	google.golang.org/api v0.243.0
	google.golang.org/grpc v1.74.2
	gopkg.in/yaml.v3 v3.0.1
)

require (
	cel.dev/expr v0.24.0 // indirect
	cloud.google.com/go v0.121.2 // indirect
	cloud.google.com/go/auth v0.16.3 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.7.0 // indirect
	cloud.google.com/go/iam v1.5.2 // indirect
	cloud.google.com/go/longrunning v0.6.7 // indirect
	cloud.google.com/go/monitoring v1.24.2 // indirect
	github.com/GoogleCloudPlatform/grpc-gcp-go/grpcgcp v1.5.3 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v1.27.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cncf/xds/go v0.0.0-20250501225837-2ac532fd4443 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/envoyproxy/go-control-plane/envoy v1.32.4 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.2.1 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-jose/go-jose/v4 v4.0.5 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.6 // indirect
	github.com/googleapis/gax-go/v2 v2.15.0 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.2.2 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/spiffe/go-spiffe/v2 v2.5.0 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/zeebo/errs v1.4.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.130.1-0.20250715222903-0a7598ec1e19 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.130.1-0.20250715222903-0a7598ec1e19 // indirect
	go.opentelemetry.io/collector/featuregate v1.36.1-0.20250715222903-0a7598ec1e19 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.130.1-0.20250715222903-0a7598ec1e19 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.130.1-0.20250715222903-0a7598ec1e19 // indirect
	go.opentelemetry.io/collector/pipeline v0.130.1-0.20250715222903-0a7598ec1e19 // indirect
	go.opentelemetry.io/collector/receiver/receiverhelper v0.130.1-0.20250715222903-0a7598ec1e19 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.130.1-0.20250715222903-0a7598ec1e19 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.12.0 // indirect
	go.opentelemetry.io/contrib/detectors/gcp v1.36.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.61.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.61.0 // indirect
	go.opentelemetry.io/otel v1.37.0 // indirect
	go.opentelemetry.io/otel/log v0.13.0 // indirect
	go.opentelemetry.io/otel/metric v1.37.0 // indirect
	go.opentelemetry.io/otel/sdk v1.37.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.37.0 // indirect
	go.opentelemetry.io/otel/trace v1.37.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.40.0 // indirect
	golang.org/x/net v0.42.0 // indirect
	golang.org/x/oauth2 v0.30.0 // indirect
	golang.org/x/sync v0.16.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
	golang.org/x/text v0.27.0 // indirect
	golang.org/x/time v0.12.0 // indirect
	google.golang.org/genproto v0.0.0-20250603155806-513f23925822 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250603155806-513f23925822 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250715232539-7130f93afb79 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
)

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)
