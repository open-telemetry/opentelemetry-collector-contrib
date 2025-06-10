module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver

go 1.23.0

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/grpcutil v0.128.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow v0.128.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent v0.128.0
	github.com/open-telemetry/otel-arrow v0.35.0
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/client v1.34.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/component v1.34.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/component/componentstatus v0.128.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/component/componenttest v0.128.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/config/configauth v0.128.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/config/configgrpc v0.128.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/config/confignet v1.34.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/config/configtls v1.34.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/confmap v1.34.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/confmap/xconfmap v0.128.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/consumer v1.34.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/consumer/consumererror v0.128.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/consumer/consumertest v0.128.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/extension v1.34.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/extension/extensionauth v1.34.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/pdata v1.34.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/receiver v1.34.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/receiver/receiverhelper v0.128.1-0.20250610090210-188191247685
	go.opentelemetry.io/collector/receiver/receivertest v0.128.1-0.20250610090210-188191247685
	go.opentelemetry.io/otel v1.36.0
	go.opentelemetry.io/otel/sdk v1.36.0
	go.opentelemetry.io/otel/sdk/metric v1.36.0
	go.opentelemetry.io/otel/trace v1.36.0
	go.uber.org/goleak v1.3.0
	go.uber.org/mock v0.5.2
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.0
	golang.org/x/net v0.40.0
	google.golang.org/grpc v1.72.2
)

require (
	github.com/HdrHistogram/hdrhistogram-go v1.1.2 // indirect
	github.com/apache/arrow-go/v18 v18.2.0 // indirect
	github.com/axiomhq/hyperloglog v0.0.0-20230201085229-3ddf4bad03dc // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgryski/go-metro v0.0.0-20180109044635-280f6062b5bc // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20250323135004-b31fac66206e // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/fxamacker/cbor/v2 v2.4.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/snappy v0.0.5-0.20220116011046-fa5810519dcb // indirect
	github.com/google/flatbuffers v25.2.10+incompatible // indirect
	github.com/google/go-tpm v0.9.5 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/klauspost/cpuid/v2 v2.2.10 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.2.0 // indirect
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
	go.opentelemetry.io/collector/config/configcompression v1.34.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/config/configmiddleware v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.34.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/exporter v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/extension/extensionmiddleware v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/featuregate v1.34.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/pipeline v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.11.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.61.0 // indirect
	go.opentelemetry.io/otel/log v0.12.2 // indirect
	go.opentelemetry.io/otel/metric v1.36.0 // indirect
	golang.org/x/crypto v0.38.0 // indirect
	golang.org/x/exp v0.0.0-20240909161429-701f63a606c0 // indirect
	golang.org/x/mod v0.23.0 // indirect
	golang.org/x/sync v0.14.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.25.0 // indirect
	golang.org/x/tools v0.30.0 // indirect
	golang.org/x/xerrors v0.0.0-20240903120638-7835f813f4da // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250519155744-55703ea1f237 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow => ../../internal/otelarrow

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent => ../../internal/sharedcomponent

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otelarrowexporter => ../../exporter/otelarrowexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/grpcutil => ../../internal/grpcutil
