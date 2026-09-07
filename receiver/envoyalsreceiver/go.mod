module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/envoyalsreceiver

go 1.26.0

require (
	github.com/envoyproxy/go-control-plane/envoy v1.39.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.160.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.160.0
	github.com/stretchr/testify v1.12.1
	go.opentelemetry.io/collector/component v1.66.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/component/componentstatus v0.160.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/component/componenttest v0.160.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/config/configgrpc v1.66.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/config/confignet v1.66.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/confmap v1.66.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/consumer v1.66.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/consumer/consumertest v0.160.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/pdata v1.66.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/receiver v1.66.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/receiver/receiverhelper v0.160.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/receiver/receivertest v0.160.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/receiver/xreceiver v0.160.1-0.20260903163450-cc4b33fc673f
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.28.0
	google.golang.org/grpc v1.83.2
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cncf/xds/go v0.0.0-20260202195803-dba9d589def2 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.3.3 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20251226215517-609e4778396f // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/go-tpm v0.9.8 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.19.2 // indirect
	github.com/knadh/koanf/maps v0.1.3 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.1 // indirect
	github.com/knadh/koanf/v2 v2.3.6 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.160.0 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/client v1.66.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/config/configauth v1.66.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/config/configcompression v1.66.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/config/configmiddleware v1.66.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/config/configopaque v1.66.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/config/configoptional v1.66.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/config/configtls v1.66.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.160.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.160.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/extension/extensionauth v1.66.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/extension/extensionmiddleware v0.160.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/featuregate v1.66.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.160.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.160.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/pdata/xpdata v0.160.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/pipeline v1.66.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.160.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.70.0 // indirect
	go.opentelemetry.io/otel v1.46.0 // indirect
	go.opentelemetry.io/otel/metric v1.46.0 // indirect
	go.opentelemetry.io/otel/sdk v1.46.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.46.0 // indirect
	go.opentelemetry.io/otel/trace v1.46.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/crypto v0.56.0 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260803160001-6ac0973c030d // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil
