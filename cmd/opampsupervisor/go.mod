module github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor

go 1.23.0

require (
	github.com/cenkalti/backoff/v4 v4.3.0
	github.com/google/uuid v1.6.0
	github.com/knadh/koanf/maps v0.1.1
	github.com/knadh/koanf/parsers/yaml v0.1.0
	github.com/knadh/koanf/providers/file v1.1.2
	github.com/knadh/koanf/providers/rawbytes v0.1.0
	github.com/knadh/koanf/v2 v2.1.2
	github.com/open-telemetry/opamp-go v0.19.0
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v1.27.0
	go.opentelemetry.io/collector/config/configopaque v1.27.0
	go.opentelemetry.io/collector/config/configtelemetry v0.121.0
	go.opentelemetry.io/collector/config/configtls v1.27.0
	go.opentelemetry.io/collector/confmap v1.27.0
	go.opentelemetry.io/collector/confmap/provider/envprovider v1.27.0
	go.opentelemetry.io/collector/confmap/provider/fileprovider v1.27.0
	go.opentelemetry.io/collector/pdata v1.27.0
	go.opentelemetry.io/collector/semconv v0.121.0
	go.opentelemetry.io/collector/service v0.121.0
	go.opentelemetry.io/contrib/config v0.14.0
	go.uber.org/goleak v1.3.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.0
	golang.org/x/sys v0.30.0
	google.golang.org/protobuf v1.36.5
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.25.1 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_golang v1.20.5 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.62.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.27.0 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.9.0 // indirect
	go.opentelemetry.io/contrib/propagators/b3 v1.34.0 // indirect
	go.opentelemetry.io/otel v1.34.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc v0.10.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.10.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.34.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.34.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.34.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.34.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.34.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.56.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutlog v0.10.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v1.34.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.34.0 // indirect
	go.opentelemetry.io/otel/log v0.10.0 // indirect
	go.opentelemetry.io/otel/metric v1.34.0 // indirect
	go.opentelemetry.io/otel/sdk v1.34.0 // indirect
	go.opentelemetry.io/otel/sdk/log v0.10.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.34.0 // indirect
	go.opentelemetry.io/otel/trace v1.34.0 // indirect
	go.opentelemetry.io/proto/otlp v1.5.0 // indirect
	golang.org/x/net v0.35.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250115164207-1a7da9e5054f // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250115164207-1a7da9e5054f // indirect
	google.golang.org/grpc v1.70.0 // indirect
)
