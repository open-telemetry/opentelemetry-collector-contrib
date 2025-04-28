module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/lokireceiver

go 1.23.0

require (
	github.com/buger/jsonparser v1.1.1
	github.com/gogo/protobuf v1.3.2
	github.com/golang/snappy v1.0.0
	github.com/grafana/loki/pkg/push v0.0.0-20240514112848-a1b1eeb09583
	github.com/json-iterator/go v1.1.12
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.124.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/loki v0.124.1
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus v0.124.1 // indirect
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v1.30.1-0.20250424234037-ac7c0f2f4cd8
	go.opentelemetry.io/collector/component/componentstatus v0.124.1-0.20250424234037-ac7c0f2f4cd8
	go.opentelemetry.io/collector/confmap v1.30.1-0.20250424234037-ac7c0f2f4cd8
	go.opentelemetry.io/collector/consumer v1.30.1-0.20250424234037-ac7c0f2f4cd8
	go.opentelemetry.io/collector/receiver v1.30.1-0.20250424234037-ac7c0f2f4cd8
	go.uber.org/zap v1.27.0
	google.golang.org/grpc v1.72.0
)

require (
	go.opentelemetry.io/collector/component/componenttest v0.124.1-0.20250424234037-ac7c0f2f4cd8
	go.opentelemetry.io/collector/config/configgrpc v0.124.1-0.20250424234037-ac7c0f2f4cd8
	go.opentelemetry.io/collector/config/confighttp v0.124.1-0.20250424234037-ac7c0f2f4cd8
	go.opentelemetry.io/collector/config/confignet v1.30.1-0.20250424234037-ac7c0f2f4cd8
	go.opentelemetry.io/collector/confmap/xconfmap v0.124.1-0.20250424234037-ac7c0f2f4cd8
	go.opentelemetry.io/collector/consumer/consumererror v0.124.1-0.20250424234037-ac7c0f2f4cd8
	go.opentelemetry.io/collector/consumer/consumertest v0.124.1-0.20250424234037-ac7c0f2f4cd8
	go.opentelemetry.io/collector/pdata v1.30.1-0.20250424234037-ac7c0f2f4cd8
	go.opentelemetry.io/collector/receiver/receiverhelper v0.124.1-0.20250424234037-ac7c0f2f4cd8
	go.opentelemetry.io/collector/receiver/receivertest v0.124.1-0.20250424234037-ac7c0f2f4cd8
	go.uber.org/goleak v1.3.0
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dennwc/varint v1.0.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grafana/regexp v0.0.0-20240518133315-a468a5bfb3bc // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.2.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mostynb/go-grpc-compression v1.2.3 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_golang v1.20.5 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.62.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/prometheus/prometheus v0.300.1 // indirect
	github.com/rs/cors v1.11.1 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/client v1.30.1-0.20250424234037-ac7c0f2f4cd8 // indirect
	go.opentelemetry.io/collector/config/configauth v0.124.1-0.20250424234037-ac7c0f2f4cd8 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.30.1-0.20250424234037-ac7c0f2f4cd8 // indirect
	go.opentelemetry.io/collector/config/configmiddleware v0.0.0-20250424234037-ac7c0f2f4cd8 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.30.1-0.20250424234037-ac7c0f2f4cd8 // indirect
	go.opentelemetry.io/collector/config/configtls v1.30.1-0.20250424234037-ac7c0f2f4cd8 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.124.1-0.20250424234037-ac7c0f2f4cd8 // indirect
	go.opentelemetry.io/collector/extension/extensionauth v1.30.1-0.20250424234037-ac7c0f2f4cd8 // indirect
	go.opentelemetry.io/collector/extension/extensionmiddleware v0.0.0-20250424234037-ac7c0f2f4cd8 // indirect
	go.opentelemetry.io/collector/featuregate v1.30.1-0.20250424234037-ac7c0f2f4cd8 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.124.1-0.20250424234037-ac7c0f2f4cd8 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.124.1-0.20250424234037-ac7c0f2f4cd8 // indirect
	go.opentelemetry.io/collector/pipeline v0.124.1-0.20250424234037-ac7c0f2f4cd8 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.124.1-0.20250424234037-ac7c0f2f4cd8 // indirect
	go.opentelemetry.io/collector/semconv v0.124.1-0.20250424234037-ac7c0f2f4cd8 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.10.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.60.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.60.0 // indirect
	go.opentelemetry.io/otel v1.35.0 // indirect
	go.opentelemetry.io/otel/log v0.11.0 // indirect
	go.opentelemetry.io/otel/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/trace v1.35.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/exp v0.0.0-20240325151524-a685a6edb6d8 // indirect
	golang.org/x/net v0.39.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250218202821-56aae31c358a // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/loki => ../../pkg/translator/loki

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus => ../../pkg/translator/prometheus

retract (
	v0.76.2
	v0.76.1
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden

replace go.opentelemetry.io/collector/extension/extensionmiddleware/extensionmiddlewaretest v0.0.0-00010101000000-000000000000 => go.opentelemetry.io/collector/extension/extensionmiddleware/extensionmiddlewaretest v0.0.0-20250424234037-ac7c0f2f4cd8

replace go.opentelemetry.io/collector/config/configmiddleware v0.0.0-00010101000000-000000000000 => go.opentelemetry.io/collector/config/configmiddleware v0.0.0-20250424234037-ac7c0f2f4cd8

replace go.opentelemetry.io/collector/extension/extensionmiddleware v1.30.0 => go.opentelemetry.io/collector/extension/extensionmiddleware v0.0.0-20250424234037-ac7c0f2f4cd8
