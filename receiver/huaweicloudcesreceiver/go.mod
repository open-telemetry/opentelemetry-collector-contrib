module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/huaweicloudcesreceiver

go 1.23.0

require (
	github.com/cenkalti/backoff/v4 v4.3.0
	github.com/huaweicloud/huaweicloud-sdk-go-v3 v0.1.161
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden v0.131.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.131.0
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v1.37.1-0.20250801020258-8b73477b9810
	go.opentelemetry.io/collector/component/componenttest v0.131.1-0.20250801020258-8b73477b9810
	go.opentelemetry.io/collector/config/confighttp v0.131.1-0.20250801020258-8b73477b9810
	go.opentelemetry.io/collector/config/configopaque v1.37.1-0.20250801020258-8b73477b9810
	go.opentelemetry.io/collector/config/configretry v1.37.1-0.20250801020258-8b73477b9810
	go.opentelemetry.io/collector/confmap v1.37.1-0.20250801020258-8b73477b9810
	go.opentelemetry.io/collector/consumer v1.37.1-0.20250801020258-8b73477b9810
	go.opentelemetry.io/collector/consumer/consumertest v0.131.1-0.20250801020258-8b73477b9810
	go.opentelemetry.io/collector/pdata v1.37.1-0.20250801020258-8b73477b9810
	go.opentelemetry.io/collector/receiver v1.37.1-0.20250801020258-8b73477b9810
	go.opentelemetry.io/collector/receiver/receivertest v0.131.1-0.20250801020258-8b73477b9810
	go.opentelemetry.io/collector/scraper/scraperhelper v0.131.1-0.20250801020258-8b73477b9810
	go.uber.org/goleak v1.3.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.0
)

require (
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fatih/color v1.10.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20250323135004-b31fac66206e // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/goccy/go-yaml v1.9.8 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/go-tpm v0.9.5 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/json-iterator/go v1.1.13-0.20220915233716-71ac16282d12 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.2.2 // indirect
	github.com/mattn/go-colorable v0.1.8 // indirect
	github.com/mattn/go-isatty v0.0.12 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.131.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/tjfoc/gmsm v1.4.1 // indirect
	go.mongodb.org/mongo-driver v1.17.1 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/client v1.37.1-0.20250801020258-8b73477b9810 // indirect
	go.opentelemetry.io/collector/config/configauth v0.131.1-0.20250801020258-8b73477b9810 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.37.1-0.20250801020258-8b73477b9810 // indirect
	go.opentelemetry.io/collector/config/configmiddleware v0.131.1-0.20250801020258-8b73477b9810 // indirect
	go.opentelemetry.io/collector/config/configoptional v0.131.1-0.20250801020258-8b73477b9810 // indirect
	go.opentelemetry.io/collector/config/configtls v1.37.1-0.20250801020258-8b73477b9810 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.131.1-0.20250801020258-8b73477b9810 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.131.1-0.20250801020258-8b73477b9810 // indirect
	go.opentelemetry.io/collector/extension/extensionauth v1.37.1-0.20250801020258-8b73477b9810 // indirect
	go.opentelemetry.io/collector/extension/extensionmiddleware v0.131.1-0.20250801020258-8b73477b9810 // indirect
	go.opentelemetry.io/collector/featuregate v1.37.1-0.20250801020258-8b73477b9810 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.131.1-0.20250801020258-8b73477b9810 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.131.1-0.20250801020258-8b73477b9810 // indirect
	go.opentelemetry.io/collector/pipeline v0.131.1-0.20250801020258-8b73477b9810 // indirect
	go.opentelemetry.io/collector/receiver/receiverhelper v0.131.1-0.20250801020258-8b73477b9810 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.131.1-0.20250801020258-8b73477b9810 // indirect
	go.opentelemetry.io/collector/scraper v0.131.1-0.20250801020258-8b73477b9810 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.12.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.62.0 // indirect
	go.opentelemetry.io/otel v1.37.0 // indirect
	go.opentelemetry.io/otel/log v0.13.0 // indirect
	go.opentelemetry.io/otel/metric v1.37.0 // indirect
	go.opentelemetry.io/otel/sdk v1.37.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.37.0 // indirect
	go.opentelemetry.io/otel/trace v1.37.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.40.0 // indirect
	golang.org/x/net v0.42.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
	golang.org/x/text v0.27.0 // indirect
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250528174236-200df99c418a // indirect
	google.golang.org/grpc v1.74.2 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden
