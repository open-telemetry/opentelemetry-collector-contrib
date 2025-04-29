module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otelarrowexporter

go 1.23.0

require (
	github.com/apache/arrow/go/v16 v16.1.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/grpcutil v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow v0.124.1
	github.com/open-telemetry/otel-arrow v0.35.0
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/client v1.30.0
	go.opentelemetry.io/collector/component v1.30.0
	go.opentelemetry.io/collector/component/componenttest v0.124.0
	go.opentelemetry.io/collector/config/configauth v0.124.0
	go.opentelemetry.io/collector/config/configcompression v1.30.0
	go.opentelemetry.io/collector/config/configgrpc v0.124.0
	go.opentelemetry.io/collector/config/configopaque v1.30.0
	go.opentelemetry.io/collector/config/configretry v1.30.0
	go.opentelemetry.io/collector/config/configtls v1.30.0
	go.opentelemetry.io/collector/confmap v1.30.0
	go.opentelemetry.io/collector/confmap/xconfmap v0.124.0
	go.opentelemetry.io/collector/consumer v1.30.0
	go.opentelemetry.io/collector/consumer/consumererror v0.124.0
	go.opentelemetry.io/collector/exporter v0.124.0
	go.opentelemetry.io/collector/exporter/exportertest v0.124.0
	go.opentelemetry.io/collector/extension v1.30.0
	go.opentelemetry.io/collector/extension/extensionauth v1.30.0
	go.opentelemetry.io/collector/pdata v1.30.0
	go.opentelemetry.io/otel v1.35.0
	go.opentelemetry.io/otel/trace v1.35.0
	go.uber.org/goleak v1.3.0
	go.uber.org/mock v0.5.1
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.0
	golang.org/x/net v0.39.0
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250218202821-56aae31c358a
	google.golang.org/grpc v1.71.1
	google.golang.org/protobuf v1.36.6
)

require (
	github.com/HdrHistogram/hdrhistogram-go v1.1.2 // indirect
	github.com/apache/arrow-go/v18 v18.2.0 // indirect
	github.com/axiomhq/hyperloglog v0.0.0-20230201085229-3ddf4bad03dc // indirect
	github.com/cenkalti/backoff/v5 v5.0.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgryski/go-metro v0.0.0-20180109044635-280f6062b5bc // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/fxamacker/cbor/v2 v2.4.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/snappy v0.0.5-0.20220116011046-fa5810519dcb // indirect
	github.com/google/flatbuffers v25.2.10+incompatible // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/klauspost/cpuid/v2 v2.2.10 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/knadh/koanf/v2 v2.1.2 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mostynb/go-grpc-compression v1.2.3 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/zeebo/xxh3 v1.0.2 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/config/confignet v1.30.0 // indirect
	go.opentelemetry.io/collector/consumer/consumertest v0.124.0 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.124.0 // indirect
	go.opentelemetry.io/collector/exporter/xexporter v0.124.0 // indirect
	go.opentelemetry.io/collector/extension/xextension v0.124.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.30.0 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.124.0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.124.0 // indirect
	go.opentelemetry.io/collector/pipeline v0.124.0 // indirect
	go.opentelemetry.io/collector/receiver v1.30.0 // indirect
	go.opentelemetry.io/collector/receiver/receivertest v0.124.0 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.124.0 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.10.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.60.0 // indirect
	go.opentelemetry.io/otel/log v0.11.0 // indirect
	go.opentelemetry.io/otel/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.35.0 // indirect
	golang.org/x/exp v0.0.0-20240909161429-701f63a606c0 // indirect
	golang.org/x/mod v0.23.0 // indirect
	golang.org/x/sync v0.13.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	golang.org/x/tools v0.30.0 // indirect
	golang.org/x/xerrors v0.0.0-20240903120638-7835f813f4da // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow => ../../internal/otelarrow

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver => ../../receiver/otelarrowreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent => ../../internal/sharedcomponent

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/grpcutil => ../../internal/grpcutil
