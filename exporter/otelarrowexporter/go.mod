module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otelarrowexporter

go 1.22.0

require (
	github.com/apache/arrow/go/v16 v16.1.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/grpcutil v0.110.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow v0.110.0
	github.com/open-telemetry/otel-arrow v0.26.0
	github.com/stretchr/testify v1.9.0
	go.opentelemetry.io/collector/client v1.16.0
	go.opentelemetry.io/collector/component v0.110.0
	go.opentelemetry.io/collector/config/configauth v0.110.0
	go.opentelemetry.io/collector/config/configcompression v1.16.0
	go.opentelemetry.io/collector/config/configgrpc v0.110.0
	go.opentelemetry.io/collector/config/configopaque v1.16.0
	go.opentelemetry.io/collector/config/configretry v1.16.0
	go.opentelemetry.io/collector/config/configtls v1.16.0
	go.opentelemetry.io/collector/confmap v1.16.0
	go.opentelemetry.io/collector/consumer v0.110.0
	go.opentelemetry.io/collector/exporter v0.110.0
	go.opentelemetry.io/collector/extension v0.110.0
	go.opentelemetry.io/collector/extension/auth v0.110.0
	go.opentelemetry.io/collector/pdata v1.16.0
	go.opentelemetry.io/otel v1.30.0
	go.opentelemetry.io/otel/trace v1.30.0
	go.uber.org/goleak v1.3.0
	go.uber.org/mock v0.4.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.0
	golang.org/x/net v0.29.0
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240903143218-8af14fe29dc1
	google.golang.org/grpc v1.66.2
	google.golang.org/protobuf v1.34.2
)

require (
	github.com/HdrHistogram/hdrhistogram-go v1.1.2 // indirect
	github.com/apache/arrow/go/v17 v17.0.0 // indirect
	github.com/axiomhq/hyperloglog v0.0.0-20230201085229-3ddf4bad03dc // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgryski/go-metro v0.0.0-20180109044635-280f6062b5bc // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/fxamacker/cbor/v2 v2.4.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.1.0 // indirect
	github.com/goccy/go-json v0.10.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/snappy v0.0.5-0.20220116011046-fa5810519dcb // indirect
	github.com/google/flatbuffers v24.3.25+incompatible // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/klauspost/cpuid/v2 v2.2.8 // indirect
	github.com/knadh/koanf/maps v0.1.1 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/knadh/koanf/v2 v2.1.1 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mostynb/go-grpc-compression v1.2.3 // indirect
	github.com/pierrec/lz4/v4 v4.1.21 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/zeebo/xxh3 v1.0.2 // indirect
	go.opentelemetry.io/collector/component/componentprofiles v0.110.0 // indirect
	go.opentelemetry.io/collector/config/confignet v1.16.0 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.110.0 // indirect
	go.opentelemetry.io/collector/config/internal v0.110.0 // indirect
	go.opentelemetry.io/collector/consumer/consumerprofiles v0.110.0 // indirect
	go.opentelemetry.io/collector/consumer/consumertest v0.110.0 // indirect
	go.opentelemetry.io/collector/exporter/exporterprofiles v0.110.0 // indirect
	go.opentelemetry.io/collector/extension/experimental/storage v0.110.0 // indirect
	go.opentelemetry.io/collector/internal/globalsignal v0.110.0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.110.0 // indirect
	go.opentelemetry.io/collector/pipeline v0.110.0 // indirect
	go.opentelemetry.io/collector/receiver v0.110.0 // indirect
	go.opentelemetry.io/collector/receiver/receiverprofiles v0.110.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.55.0 // indirect
	go.opentelemetry.io/otel/metric v1.30.0 // indirect
	go.opentelemetry.io/otel/sdk v1.30.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.30.0 // indirect
	golang.org/x/exp v0.0.0-20240506185415-9bf2ced13842 // indirect
	golang.org/x/mod v0.18.0 // indirect
	golang.org/x/sync v0.8.0 // indirect
	golang.org/x/sys v0.25.0 // indirect
	golang.org/x/text v0.18.0 // indirect
	golang.org/x/tools v0.22.0 // indirect
	golang.org/x/xerrors v0.0.0-20231012003039-104605ab7028 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow => ../../internal/otelarrow

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver => ../../receiver/otelarrowreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent => ../../internal/sharedcomponent

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/grpcutil => ../../internal/grpcutil
