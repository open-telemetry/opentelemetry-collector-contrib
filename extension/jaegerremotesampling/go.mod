module github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling

go 1.20

require (
	github.com/fortytw2/leaktest v1.3.0
	github.com/jaegertracing/jaeger v1.53.0
	github.com/stretchr/testify v1.8.4
	github.com/tilinna/clock v1.1.0
	go.opentelemetry.io/collector/component v0.92.1-0.20240118172122-8131d31601b8
	go.opentelemetry.io/collector/config/configgrpc v0.92.1-0.20240118172122-8131d31601b8
	go.opentelemetry.io/collector/config/confighttp v0.92.1-0.20240118172122-8131d31601b8
	go.opentelemetry.io/collector/config/confignet v0.92.1-0.20240118172122-8131d31601b8
	go.opentelemetry.io/collector/config/configopaque v0.92.1-0.20240118172122-8131d31601b8
	go.opentelemetry.io/collector/config/configtls v0.92.1-0.20240118172122-8131d31601b8
	go.opentelemetry.io/collector/confmap v0.92.1-0.20240118172122-8131d31601b8
	go.opentelemetry.io/collector/extension v0.92.1-0.20240118172122-8131d31601b8
	go.opentelemetry.io/collector/featuregate v1.0.2-0.20240118172122-8131d31601b8
	go.opentelemetry.io/otel/metric v1.22.0
	go.opentelemetry.io/otel/trace v1.22.0
	go.uber.org/zap v1.26.0
	google.golang.org/grpc v1.60.1
)

require (
	contrib.go.opencensus.io/exporter/prometheus v0.4.2 // indirect
	github.com/apache/thrift v0.19.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logfmt/logfmt v0.5.1 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gogo/googleapis v1.4.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/hashicorp/go-version v1.6.0 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/klauspost/compress v1.17.4 // indirect
	github.com/knadh/koanf/maps v0.1.1 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/knadh/koanf/v2 v2.0.1 // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.1-0.20231216201459-8508981c8b6c // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/mostynb/go-grpc-compression v1.2.2 // indirect
	github.com/pelletier/go-toml/v2 v2.1.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_golang v1.18.0 // indirect
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/common v0.46.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/prometheus/statsd_exporter v0.22.7 // indirect
	github.com/rs/cors v1.10.1 // indirect
	github.com/sagikazarmark/locafero v0.4.0 // indirect
	github.com/sagikazarmark/slog-shim v0.1.0 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spf13/afero v1.11.0 // indirect
	github.com/spf13/cast v1.6.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/spf13/viper v1.18.2 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/collector v0.92.1-0.20240118172122-8131d31601b8 // indirect
	go.opentelemetry.io/collector/config/configauth v0.92.1-0.20240118172122-8131d31601b8 // indirect
	go.opentelemetry.io/collector/config/configcompression v0.92.1-0.20240118172122-8131d31601b8 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.92.1-0.20240118172122-8131d31601b8 // indirect
	go.opentelemetry.io/collector/config/internal v0.92.1-0.20240118172122-8131d31601b8 // indirect
	go.opentelemetry.io/collector/extension/auth v0.92.1-0.20240118172122-8131d31601b8 // indirect
	go.opentelemetry.io/collector/pdata v1.0.2-0.20240118172122-8131d31601b8 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.47.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.46.1 // indirect
	go.opentelemetry.io/otel v1.22.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.44.1-0.20231201153405-6027c1ae76f2 // indirect
	go.opentelemetry.io/otel/sdk v1.22.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.22.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/exp v0.0.0-20231127185646-65229373498e // indirect
	golang.org/x/net v0.20.0 // indirect
	golang.org/x/sys v0.16.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20231127180814-3a041ad873d4 // indirect
	google.golang.org/protobuf v1.32.0 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)
