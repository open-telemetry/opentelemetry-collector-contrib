module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/envoyalsreceiver

go 1.23.0

require (
	github.com/envoyproxy/go-control-plane/envoy v1.32.4
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.120.1
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.120.1
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v0.120.1-0.20250226024140-8099e51f9a77
	go.opentelemetry.io/collector/component/componentstatus v0.120.1-0.20250226024140-8099e51f9a77
	go.opentelemetry.io/collector/component/componenttest v0.120.1-0.20250226024140-8099e51f9a77
	go.opentelemetry.io/collector/config/configgrpc v0.120.1-0.20250226024140-8099e51f9a77
	go.opentelemetry.io/collector/config/confignet v1.26.1-0.20250226024140-8099e51f9a77
	go.opentelemetry.io/collector/confmap v1.26.1-0.20250226024140-8099e51f9a77
	go.opentelemetry.io/collector/confmap/xconfmap v0.120.1-0.20250226024140-8099e51f9a77
	go.opentelemetry.io/collector/consumer v1.26.1-0.20250226024140-8099e51f9a77
	go.opentelemetry.io/collector/consumer/consumertest v0.120.1-0.20250226024140-8099e51f9a77
	go.opentelemetry.io/collector/pdata v1.26.1-0.20250226024140-8099e51f9a77
	go.opentelemetry.io/collector/receiver v0.120.1-0.20250226024140-8099e51f9a77
	go.opentelemetry.io/collector/receiver/receivertest v0.120.1-0.20250226024140-8099e51f9a77
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.0
	google.golang.org/grpc v1.70.0
	google.golang.org/protobuf v1.36.5
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cncf/xds/go v0.0.0-20240905190251-b4127c9b8d78 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.2.1 // indirect
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/uuid v1.6.0 // indirect
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
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.120.1 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/client v1.26.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/config/configauth v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.26.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.26.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/config/configtls v1.26.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/extension v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/extension/extensionauth v0.120.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.26.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/pipeline v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.59.0 // indirect
	go.opentelemetry.io/otel v1.34.0 // indirect
	go.opentelemetry.io/otel/metric v1.34.0 // indirect
	go.opentelemetry.io/otel/sdk v1.34.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.34.0 // indirect
	go.opentelemetry.io/otel/trace v1.34.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/sys v0.29.0 // indirect
	golang.org/x/text v0.21.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250115164207-1a7da9e5054f // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace go.opentelemetry.io/collector/extension/extensionauth => go.opentelemetry.io/collector/extension/extensionauth v0.0.0-20250226024140-8099e51f9a77

replace go.opentelemetry.io/collector/extension/extensionauth/extensionauthtest => go.opentelemetry.io/collector/extension/extensionauth/extensionauthtest v0.0.0-20250226024140-8099e51f9a77

replace go.opentelemetry.io/collector/service/hostcapabilities => go.opentelemetry.io/collector/service/hostcapabilities v0.0.0-20250226024140-8099e51f9a77
