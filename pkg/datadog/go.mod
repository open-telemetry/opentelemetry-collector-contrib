module github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog

go 1.22.0

require (
	github.com/DataDog/datadog-agent/pkg/util/hostname/validate v0.61.0
	github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/metrics v0.24.0
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v0.118.1-0.20250121185328-fbefb22cc2b3
	go.opentelemetry.io/collector/component/componenttest v0.118.1-0.20250121185328-fbefb22cc2b3
	go.opentelemetry.io/collector/config/configauth v0.118.1-0.20250121185328-fbefb22cc2b3
	go.opentelemetry.io/collector/config/confighttp v0.118.1-0.20250121185328-fbefb22cc2b3
	go.opentelemetry.io/collector/config/confignet v1.24.1-0.20250123125445-24f88da7b583
	go.opentelemetry.io/collector/config/configopaque v1.24.1-0.20250123125445-24f88da7b583
	go.opentelemetry.io/collector/config/configretry v1.24.1-0.20250123125445-24f88da7b583
	go.opentelemetry.io/collector/config/configtls v1.24.1-0.20250123125445-24f88da7b583
	go.opentelemetry.io/collector/confmap v1.24.1-0.20250123125445-24f88da7b583
	go.opentelemetry.io/collector/exporter v0.118.1-0.20250121185328-fbefb22cc2b3
	go.opentelemetry.io/collector/featuregate v1.24.1-0.20250123125445-24f88da7b583
	go.uber.org/zap v1.27.0
)

require (
	github.com/DataDog/datadog-agent/pkg/proto v0.63.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/util/log v0.61.0 // indirect
	github.com/DataDog/datadog-agent/pkg/util/scrubber v0.61.0 // indirect
	github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes v0.24.0 // indirect
	github.com/DataDog/opentelemetry-mapping-go/pkg/quantile v0.24.0 // indirect
	github.com/DataDog/sketches-go v1.4.6 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cihub/seelog v0.0.0-20170130134532-f561c5e57575 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/knadh/koanf/maps v0.1.1 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/knadh/koanf/v2 v2.1.2 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/philhofer/fwd v1.1.3-0.20240916144458-20a13a1f6b7c // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/tinylib/msgp v1.2.5 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/client v1.24.1-0.20250123125445-24f88da7b583 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.24.1-0.20250123125445-24f88da7b583 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.118.1-0.20250121185328-fbefb22cc2b3 // indirect
	go.opentelemetry.io/collector/consumer v1.24.1-0.20250123125445-24f88da7b583 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.118.1-0.20250121185328-fbefb22cc2b3 // indirect
	go.opentelemetry.io/collector/extension v0.118.1-0.20250121185328-fbefb22cc2b3 // indirect
	go.opentelemetry.io/collector/extension/auth v0.118.1-0.20250121185328-fbefb22cc2b3 // indirect
	go.opentelemetry.io/collector/extension/xextension v0.118.1-0.20250121185328-fbefb22cc2b3 // indirect
	go.opentelemetry.io/collector/pdata v1.24.1-0.20250123125445-24f88da7b583 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.118.1-0.20250121185328-fbefb22cc2b3 // indirect
	go.opentelemetry.io/collector/pipeline v0.118.1-0.20250121185328-fbefb22cc2b3 // indirect
	go.opentelemetry.io/collector/semconv v0.118.1-0.20250121185328-fbefb22cc2b3 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.56.0 // indirect
	go.opentelemetry.io/otel v1.34.0 // indirect
	go.opentelemetry.io/otel/metric v1.34.0 // indirect
	go.opentelemetry.io/otel/sdk v1.34.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.34.0 // indirect
	go.opentelemetry.io/otel/trace v1.34.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/exp v0.0.0-20241217172543-b2144cdd0a67 // indirect
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241223144023-3abc09e42ca8 // indirect
	google.golang.org/grpc v1.69.4 // indirect
	google.golang.org/protobuf v1.36.3 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
