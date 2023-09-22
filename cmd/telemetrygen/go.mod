module github.com/open-telemetry/opentelemetry-collector-contrib/cmd/telemetrygen

go 1.20

require (
	github.com/grpc-ecosystem/go-grpc-middleware v1.4.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.85.0
	github.com/spf13/cobra v1.7.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.8.4
	go.opentelemetry.io/collector/component v0.85.1-0.20230922175119-921b6125f017
	go.opentelemetry.io/collector/consumer v0.85.1-0.20230922175119-921b6125f017
	go.opentelemetry.io/collector/pdata v1.0.0-rcv0014.0.20230922175119-921b6125f017
	go.opentelemetry.io/collector/receiver v0.85.1-0.20230922175119-921b6125f017
	go.opentelemetry.io/collector/receiver/otlpreceiver v0.85.1-0.20230922175119-921b6125f017
	go.opentelemetry.io/collector/semconv v0.85.1-0.20230922175119-921b6125f017
	go.opentelemetry.io/otel v1.18.0
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v0.41.0
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v0.41.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.18.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.18.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.18.0
	go.opentelemetry.io/otel/sdk v1.18.0
	go.opentelemetry.io/otel/sdk/metric v0.41.0
	go.opentelemetry.io/otel/trace v1.18.0
	go.uber.org/zap v1.26.0
	golang.org/x/time v0.3.0
	google.golang.org/grpc v1.58.1
)

require (
	github.com/cenkalti/backoff/v4 v4.2.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.18.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.17.0 // indirect
	github.com/knadh/koanf v1.5.0 // indirect
	github.com/knadh/koanf/v2 v2.0.1 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.1-0.20220423185008-bf980b35cac4 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mostynb/go-grpc-compression v1.2.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rs/cors v1.10.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/collector v0.85.0 // indirect
	go.opentelemetry.io/collector/config/configauth v0.85.1-0.20230922175119-921b6125f017 // indirect
	go.opentelemetry.io/collector/config/configcompression v0.85.1-0.20230922175119-921b6125f017 // indirect
	go.opentelemetry.io/collector/config/configgrpc v0.85.1-0.20230922175119-921b6125f017 // indirect
	go.opentelemetry.io/collector/config/confighttp v0.85.1-0.20230922175119-921b6125f017 // indirect
	go.opentelemetry.io/collector/config/confignet v0.85.1-0.20230922175119-921b6125f017 // indirect
	go.opentelemetry.io/collector/config/configopaque v0.85.1-0.20230922175119-921b6125f017 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.85.1-0.20230922175119-921b6125f017 // indirect
	go.opentelemetry.io/collector/config/configtls v0.85.1-0.20230922175119-921b6125f017 // indirect
	go.opentelemetry.io/collector/config/internal v0.85.1-0.20230922175119-921b6125f017 // indirect
	go.opentelemetry.io/collector/confmap v0.85.1-0.20230922175119-921b6125f017 // indirect
	go.opentelemetry.io/collector/exporter v0.85.1-0.20230922175119-921b6125f017 // indirect
	go.opentelemetry.io/collector/extension v0.85.1-0.20230922175119-921b6125f017 // indirect
	go.opentelemetry.io/collector/extension/auth v0.85.1-0.20230922175119-921b6125f017 // indirect
	go.opentelemetry.io/collector/featuregate v1.0.0-rcv0014.0.20230922175119-921b6125f017 // indirect
	go.opentelemetry.io/collector/processor v0.85.1-0.20230922175119-921b6125f017 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.44.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.44.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric v0.41.0 // indirect
	go.opentelemetry.io/otel/metric v1.18.0 // indirect
	go.opentelemetry.io/proto/otlp v1.0.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.15.0 // indirect
	golang.org/x/sys v0.12.0 // indirect
	golang.org/x/text v0.13.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20230822172742-b8732ec3820d // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230822172742-b8732ec3820d // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

// ambiguous import: found package cloud.google.com/go/compute/metadata in multiple modules
replace cloud.google.com/go => cloud.google.com/go v0.110.2
