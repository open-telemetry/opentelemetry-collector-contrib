module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/envoyalsreceiver

go 1.23.0

require (
	github.com/envoyproxy/go-control-plane/envoy v1.32.4
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.126.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.126.0
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v1.32.1-0.20250523042642-a867641d12bd
	go.opentelemetry.io/collector/component/componentstatus v0.126.1-0.20250523042642-a867641d12bd
	go.opentelemetry.io/collector/component/componenttest v0.126.1-0.20250523042642-a867641d12bd
	go.opentelemetry.io/collector/config/configgrpc v0.126.1-0.20250523042642-a867641d12bd
	go.opentelemetry.io/collector/config/confignet v1.32.1-0.20250523042642-a867641d12bd
	go.opentelemetry.io/collector/confmap v1.32.1-0.20250523042642-a867641d12bd
	go.opentelemetry.io/collector/confmap/xconfmap v0.126.1-0.20250523042642-a867641d12bd
	go.opentelemetry.io/collector/consumer v1.32.1-0.20250523042642-a867641d12bd
	go.opentelemetry.io/collector/consumer/consumertest v0.126.1-0.20250523042642-a867641d12bd
	go.opentelemetry.io/collector/pdata v1.32.1-0.20250523042642-a867641d12bd
	go.opentelemetry.io/collector/receiver v1.32.1-0.20250523042642-a867641d12bd
	go.opentelemetry.io/collector/receiver/receiverhelper v0.126.1-0.20250523042642-a867641d12bd
	go.opentelemetry.io/collector/receiver/receivertest v0.126.1-0.20250523042642-a867641d12bd
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.0
	google.golang.org/grpc v1.72.1
	google.golang.org/protobuf v1.36.6
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cncf/xds/go v0.0.0-20250121191232-2f005788dc42 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.2.1 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20250323135004-b31fac66206e // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/go-tpm v0.9.5 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.2.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mostynb/go-grpc-compression v1.2.3 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.126.0 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/client v1.32.1-0.20250523042642-a867641d12bd // indirect
	go.opentelemetry.io/collector/config/configauth v0.126.1-0.20250523042642-a867641d12bd // indirect
	go.opentelemetry.io/collector/config/configcompression v1.32.1-0.20250523042642-a867641d12bd // indirect
	go.opentelemetry.io/collector/config/configmiddleware v0.126.1-0.20250523042642-a867641d12bd // indirect
	go.opentelemetry.io/collector/config/configopaque v1.32.1-0.20250523042642-a867641d12bd // indirect
	go.opentelemetry.io/collector/config/configtls v1.32.1-0.20250523042642-a867641d12bd // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.126.1-0.20250523042642-a867641d12bd // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.126.1-0.20250523042642-a867641d12bd // indirect
	go.opentelemetry.io/collector/extension/extensionauth v1.32.1-0.20250523042642-a867641d12bd // indirect
	go.opentelemetry.io/collector/extension/extensionmiddleware v0.126.1-0.20250523042642-a867641d12bd // indirect
	go.opentelemetry.io/collector/featuregate v1.32.1-0.20250523042642-a867641d12bd // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.126.1-0.20250523042642-a867641d12bd // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.126.1-0.20250523042642-a867641d12bd // indirect
	go.opentelemetry.io/collector/pipeline v0.126.1-0.20250523042642-a867641d12bd // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.126.1-0.20250523042642-a867641d12bd // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.10.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.60.0 // indirect
	go.opentelemetry.io/otel v1.35.0 // indirect
	go.opentelemetry.io/otel/log v0.11.0 // indirect
	go.opentelemetry.io/otel/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/trace v1.35.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/net v0.39.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250218202821-56aae31c358a // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil
