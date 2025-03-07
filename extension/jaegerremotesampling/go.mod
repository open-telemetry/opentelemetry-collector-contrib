module github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling

go 1.23.0

require (
	github.com/fortytw2/leaktest v1.3.0
	github.com/jaegertracing/jaeger-idl v0.5.0
	github.com/jonboulle/clockwork v0.5.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.121.0
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v1.27.1-0.20250307164521-7c787571daa5
	go.opentelemetry.io/collector/component/componentstatus v0.121.1-0.20250307145831-dc9250a6c150
	go.opentelemetry.io/collector/component/componenttest v0.121.1-0.20250307145831-dc9250a6c150
	go.opentelemetry.io/collector/config/configgrpc v0.121.1-0.20250307145831-dc9250a6c150
	go.opentelemetry.io/collector/config/confighttp v0.121.1-0.20250307145831-dc9250a6c150
	go.opentelemetry.io/collector/config/confignet v1.27.1-0.20250307164521-7c787571daa5
	go.opentelemetry.io/collector/config/configopaque v1.27.1-0.20250307164521-7c787571daa5
	go.opentelemetry.io/collector/config/configtls v1.27.1-0.20250307164521-7c787571daa5
	go.opentelemetry.io/collector/confmap v1.27.1-0.20250307164521-7c787571daa5
	go.opentelemetry.io/collector/confmap/xconfmap v0.121.1-0.20250307164521-7c787571daa5
	go.opentelemetry.io/collector/extension v1.27.1-0.20250307164521-7c787571daa5
	go.opentelemetry.io/collector/extension/extensiontest v0.121.1-0.20250307145831-dc9250a6c150
	go.opentelemetry.io/collector/featuregate v1.27.1-0.20250307164521-7c787571daa5
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.0
	google.golang.org/grpc v1.71.0
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gogo/googleapis v1.4.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/knadh/koanf/maps v0.1.1 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/knadh/koanf/v2 v2.1.2 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/mostynb/go-grpc-compression v1.2.3 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/rs/cors v1.11.1 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/client v1.27.1-0.20250307164521-7c787571daa5 // indirect
	go.opentelemetry.io/collector/config/configauth v0.121.1-0.20250307145831-dc9250a6c150 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.27.1-0.20250307164521-7c787571daa5 // indirect
	go.opentelemetry.io/collector/extension/extensionauth v0.121.1-0.20250307145831-dc9250a6c150 // indirect
	go.opentelemetry.io/collector/pdata v1.27.1-0.20250307164521-7c787571daa5 // indirect
	go.opentelemetry.io/collector/pipeline v0.121.1-0.20250307145831-dc9250a6c150 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.60.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.60.0 // indirect
	go.opentelemetry.io/otel v1.35.0 // indirect
	go.opentelemetry.io/otel/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/trace v1.35.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.36.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250218202821-56aae31c358a // indirect
	google.golang.org/protobuf v1.36.5 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common
