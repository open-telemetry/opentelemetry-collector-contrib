module github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen/internal/e2etest

go 1.23.0

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen v0.119.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.119.0
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component/componenttest v0.119.1-0.20250210123122-44b3eeda354c
	go.opentelemetry.io/collector/consumer/consumertest v0.119.1-0.20250210123122-44b3eeda354c
	go.opentelemetry.io/collector/receiver/otlpreceiver v0.119.1-0.20250210123122-44b3eeda354c
	go.opentelemetry.io/collector/receiver/receivertest v0.119.1-0.20250210123122-44b3eeda354c
)

require (
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.25.1 // indirect
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
	github.com/mostynb/go-grpc-compression v1.2.3 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector v0.119.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/client v1.25.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/component v0.119.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/component/componentattribute v0.0.0-20250207221750-83d93cd7cf86 // indirect
	go.opentelemetry.io/collector/component/componentstatus v0.119.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/config/configauth v0.119.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/config/configcompression v1.25.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/config/configgrpc v0.119.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/config/confighttp v0.119.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/config/confignet v1.25.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/config/configopaque v1.25.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/config/configtls v1.25.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/confmap v1.25.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/consumer v1.25.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.119.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.119.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/extension v0.119.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/extension/auth v0.119.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/featuregate v1.25.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/internal/sharedcomponent v0.119.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/pdata v1.25.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.119.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/pipeline v0.119.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/receiver v0.119.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.119.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/collector/semconv v0.119.1-0.20250210123122-44b3eeda354c // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.59.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.59.0 // indirect
	go.opentelemetry.io/otel v1.34.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc v0.10.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.10.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.34.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.34.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.34.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.34.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.34.0 // indirect
	go.opentelemetry.io/otel/log v0.10.0 // indirect
	go.opentelemetry.io/otel/metric v1.34.0 // indirect
	go.opentelemetry.io/otel/sdk v1.34.0 // indirect
	go.opentelemetry.io/otel/sdk/log v0.10.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.34.0 // indirect
	go.opentelemetry.io/otel/trace v1.34.0 // indirect
	go.opentelemetry.io/proto/otlp v1.5.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	golang.org/x/time v0.10.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250115164207-1a7da9e5054f // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250115164207-1a7da9e5054f // indirect
	google.golang.org/grpc v1.70.0 // indirect
	google.golang.org/protobuf v1.36.5 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../../../internal/common

replace github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen => ../..
