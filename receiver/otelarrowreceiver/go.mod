module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver

go 1.20

require (
	github.com/open-telemetry/otel-arrow/collector v0.4.0
	go.opentelemetry.io/collector/component v0.84.0
	go.opentelemetry.io/collector/confmap v0.84.0
	go.opentelemetry.io/collector/receiver v0.84.0
)

require (
	github.com/HdrHistogram/hdrhistogram-go v1.1.2 // indirect
	github.com/andybalholm/brotli v1.0.5 // indirect
	github.com/apache/arrow/go/v12 v12.0.1 // indirect
	github.com/apache/thrift v0.16.0 // indirect
	github.com/axiomhq/hyperloglog v0.0.0-20230201085229-3ddf4bad03dc // indirect
	github.com/dgryski/go-metro v0.0.0-20180109044635-280f6062b5bc // indirect
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/fxamacker/cbor/v2 v2.4.0 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/goccy/go-json v0.9.11 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/golang/snappy v0.0.5-0.20220116011046-fa5810519dcb // indirect
	github.com/google/flatbuffers v2.0.8+incompatible // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/asmfmt v1.3.2 // indirect
	github.com/klauspost/compress v1.16.7 // indirect
	github.com/klauspost/cpuid/v2 v2.0.9 // indirect
	github.com/knadh/koanf v1.5.0 // indirect
	github.com/knadh/koanf/v2 v2.0.1 // indirect
	github.com/minio/asm2plan9s v0.0.0-20200509001527-cdd76441f9d8 // indirect
	github.com/minio/c2goasm v0.0.0-20190812172519-36a3d3bbc4f3 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.1-0.20220423185008-bf980b35cac4 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mostynb/go-grpc-compression v1.2.0 // indirect
	github.com/open-telemetry/otel-arrow v0.4.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.17 // indirect
	github.com/rs/cors v1.9.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/zeebo/xxh3 v1.0.2 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/collector v0.84.0 // indirect
	go.opentelemetry.io/collector/config/configauth v0.84.0 // indirect
	go.opentelemetry.io/collector/config/configcompression v0.84.0 // indirect
	go.opentelemetry.io/collector/config/configgrpc v0.84.0 // indirect
	go.opentelemetry.io/collector/config/confighttp v0.84.0 // indirect
	go.opentelemetry.io/collector/config/confignet v0.84.0 // indirect
	go.opentelemetry.io/collector/config/configopaque v0.84.0 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.84.0 // indirect
	go.opentelemetry.io/collector/config/configtls v0.84.0 // indirect
	go.opentelemetry.io/collector/config/internal v0.84.0 // indirect
	go.opentelemetry.io/collector/consumer v0.84.0 // indirect
	go.opentelemetry.io/collector/exporter v0.84.0 // indirect
	go.opentelemetry.io/collector/extension v0.84.0 // indirect
	go.opentelemetry.io/collector/extension/auth v0.84.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.0.0-rcv0014 // indirect
	go.opentelemetry.io/collector/pdata v1.0.0-rcv0014 // indirect
	go.opentelemetry.io/collector/processor v0.84.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.42.1-0.20230612162650-64be7e574a17 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.42.0 // indirect
	go.opentelemetry.io/otel v1.16.0 // indirect
	go.opentelemetry.io/otel/metric v1.16.0 // indirect
	go.opentelemetry.io/otel/trace v1.16.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.25.0 // indirect
	golang.org/x/mod v0.9.0 // indirect
	golang.org/x/net v0.14.0 // indirect
	golang.org/x/oauth2 v0.8.0 // indirect
	golang.org/x/sys v0.11.0 // indirect
	golang.org/x/text v0.12.0 // indirect
	golang.org/x/tools v0.7.0 // indirect
	golang.org/x/xerrors v0.0.0-20220907171357-04be3eba64a2 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230530153820-e85fd2cbaebc // indirect
	google.golang.org/grpc v1.57.0 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
)

// irrelevent ambiguous import problem
replace cloud.google.com/go => cloud.google.com/go v0.110.2
