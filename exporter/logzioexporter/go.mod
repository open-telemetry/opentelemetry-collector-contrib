module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logzioexporter

go 1.23.0

require (
	github.com/gogo/protobuf v1.3.2
	github.com/hashicorp/go-hclog v1.6.3
	github.com/jaegertracing/jaeger v1.66.0
	github.com/jaegertracing/jaeger-idl v0.5.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger v0.121.0
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v1.27.1-0.20250307194215-7d3e03e500b0
	go.opentelemetry.io/collector/component/componenttest v0.121.1-0.20250307194215-7d3e03e500b0
	go.opentelemetry.io/collector/config/configcompression v1.27.1-0.20250307194215-7d3e03e500b0
	go.opentelemetry.io/collector/config/confighttp v0.121.1-0.20250307194215-7d3e03e500b0
	go.opentelemetry.io/collector/config/configopaque v1.27.1-0.20250307194215-7d3e03e500b0
	go.opentelemetry.io/collector/config/configretry v1.27.1-0.20250307194215-7d3e03e500b0
	go.opentelemetry.io/collector/confmap v1.27.1-0.20250307194215-7d3e03e500b0
	go.opentelemetry.io/collector/consumer/consumererror v0.121.1-0.20250307194215-7d3e03e500b0
	go.opentelemetry.io/collector/exporter v0.121.1-0.20250307194215-7d3e03e500b0
	go.opentelemetry.io/collector/exporter/exportertest v0.121.1-0.20250307194215-7d3e03e500b0
	go.opentelemetry.io/collector/pdata v1.27.1-0.20250307194215-7d3e03e500b0
	go.opentelemetry.io/collector/pdata/testdata v0.121.1-0.20250307194215-7d3e03e500b0
	go.opentelemetry.io/collector/semconv v0.121.1-0.20250307194215-7d3e03e500b0
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.0
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250115164207-1a7da9e5054f
	google.golang.org/protobuf v1.36.5
)

require (
	github.com/apache/thrift v0.21.0 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/fatih/color v1.15.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/knadh/koanf/maps v0.1.1 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/knadh/koanf/v2 v2.1.2 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.121.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/core/xidutils v0.121.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/rs/cors v1.11.1 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/client v1.27.1-0.20250307194215-7d3e03e500b0 // indirect
	go.opentelemetry.io/collector/config/configauth v0.121.1-0.20250307194215-7d3e03e500b0 // indirect
	go.opentelemetry.io/collector/config/configtls v1.27.1-0.20250307194215-7d3e03e500b0 // indirect
	go.opentelemetry.io/collector/consumer v1.27.1-0.20250307194215-7d3e03e500b0 // indirect
	go.opentelemetry.io/collector/consumer/consumertest v0.121.1-0.20250307194215-7d3e03e500b0 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.121.1-0.20250307194215-7d3e03e500b0 // indirect
	go.opentelemetry.io/collector/exporter/xexporter v0.121.1-0.20250307194215-7d3e03e500b0 // indirect
	go.opentelemetry.io/collector/extension v1.27.1-0.20250307194215-7d3e03e500b0 // indirect
	go.opentelemetry.io/collector/extension/extensionauth v0.121.1-0.20250307194215-7d3e03e500b0 // indirect
	go.opentelemetry.io/collector/extension/xextension v0.121.1-0.20250307194215-7d3e03e500b0 // indirect
	go.opentelemetry.io/collector/featuregate v1.27.1-0.20250307194215-7d3e03e500b0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.121.1-0.20250307194215-7d3e03e500b0 // indirect
	go.opentelemetry.io/collector/pipeline v0.121.1-0.20250307194215-7d3e03e500b0 // indirect
	go.opentelemetry.io/collector/receiver v0.121.1-0.20250307194215-7d3e03e500b0 // indirect
	go.opentelemetry.io/collector/receiver/receivertest v0.121.1-0.20250307194215-7d3e03e500b0 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.121.1-0.20250307194215-7d3e03e500b0 // indirect
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
	google.golang.org/grpc v1.71.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger => ../../pkg/translator/jaeger

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/core/xidutils => ../../pkg/core/xidutils

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden
