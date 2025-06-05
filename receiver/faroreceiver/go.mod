module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/faroreceiver

go 1.23.3

require (
	github.com/grafana/faro/pkg/go v0.0.0-20250314155512-06a06da3b8bc
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component/componentstatus v0.127.1-0.20250605124916-efac6c7d44f1
	go.opentelemetry.io/collector/config/confighttp v0.127.1-0.20250605124916-efac6c7d44f1
	go.opentelemetry.io/collector/consumer/consumertest v0.127.1-0.20250605124916-efac6c7d44f1
	go.opentelemetry.io/collector/receiver/receiverhelper v0.127.1-0.20250605124916-efac6c7d44f1
	go.opentelemetry.io/collector/receiver/receivertest v0.127.1-0.20250605124916-efac6c7d44f1
	go.uber.org/goleak v1.3.0
)

require go.opentelemetry.io/collector/extension/extensionauth v1.33.1-0.20250605124916-efac6c7d44f1 // indirect

require (
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/buger/jsonparser v1.1.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20250323135004-b31fac66206e // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/google/go-tpm v0.9.5 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/cpuid/v2 v2.2.9 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.2.0 // indirect
	github.com/mailru/easyjson v0.9.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/oapi-codegen/runtime v1.1.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.127.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/wk8/go-ordered-map/v2 v2.1.8 // indirect
	github.com/zeebo/xxh3 v1.0.2 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/config/configmiddleware v0.127.1-0.20250605124916-efac6c7d44f1 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.127.1-0.20250605124916-efac6c7d44f1 // indirect
	go.opentelemetry.io/collector/extension/extensionmiddleware v0.127.1-0.20250605124916-efac6c7d44f1 // indirect
	go.opentelemetry.io/collector/featuregate v1.33.1-0.20250605124916-efac6c7d44f1 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.127.1-0.20250605124916-efac6c7d44f1 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.127.1-0.20250605124916-efac6c7d44f1 // indirect
	go.opentelemetry.io/collector/pipeline v0.127.1-0.20250605124916-efac6c7d44f1 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.127.1-0.20250605124916-efac6c7d44f1 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.11.0 // indirect
	go.opentelemetry.io/otel/log v0.12.2 // indirect
	go.opentelemetry.io/otel/sdk v1.36.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.36.0 // indirect
	golang.org/x/crypto v0.38.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

require (
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent v0.127.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden v0.127.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.127.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/faro v0.127.0
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/rs/cors v1.11.1 // indirect
	go.opentelemetry.io/collector/client v1.33.1-0.20250605124916-efac6c7d44f1 // indirect
	go.opentelemetry.io/collector/component v1.33.1-0.20250605124916-efac6c7d44f1
	go.opentelemetry.io/collector/component/componenttest v0.127.1-0.20250605124916-efac6c7d44f1
	go.opentelemetry.io/collector/config/configauth v0.127.1-0.20250605124916-efac6c7d44f1 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.33.1-0.20250605124916-efac6c7d44f1 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.33.1-0.20250605124916-efac6c7d44f1 // indirect
	go.opentelemetry.io/collector/config/configtls v1.33.1-0.20250605124916-efac6c7d44f1
	go.opentelemetry.io/collector/confmap v1.33.1-0.20250605124916-efac6c7d44f1
	go.opentelemetry.io/collector/consumer v1.33.1-0.20250605124916-efac6c7d44f1
	go.opentelemetry.io/collector/consumer/xconsumer v0.127.1-0.20250605124916-efac6c7d44f1 // indirect
	go.opentelemetry.io/collector/pdata v1.33.1-0.20250605124916-efac6c7d44f1
	go.opentelemetry.io/collector/receiver v1.33.1-0.20250605124916-efac6c7d44f1
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.61.0 // indirect
	go.opentelemetry.io/otel v1.36.0 // indirect
	go.opentelemetry.io/otel/metric v1.36.0 // indirect
	go.opentelemetry.io/otel/trace v1.36.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0
	golang.org/x/net v0.40.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.25.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250324211829-b45e905df463 // indirect
	google.golang.org/grpc v1.72.2 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent => ../../internal/sharedcomponent

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/faro => ../../pkg/translator/faro
