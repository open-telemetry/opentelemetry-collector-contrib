module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/oracledbreceiver

go 1.23.0

require (
	github.com/DataDog/datadog-agent/pkg/obfuscate v0.68.0
	github.com/google/go-cmp v0.7.0
	github.com/hashicorp/golang-lru/v2 v2.0.7
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden v0.131.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.131.0
	github.com/sijms/go-ora/v2 v2.9.0
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v1.37.1-0.20250801020258-8b73477b9810
	go.opentelemetry.io/collector/component/componenttest v0.131.1-0.20250801020258-8b73477b9810
	go.opentelemetry.io/collector/confmap v1.37.1-0.20250801020258-8b73477b9810
	go.opentelemetry.io/collector/consumer v1.37.1-0.20250801020258-8b73477b9810
	go.opentelemetry.io/collector/consumer/consumertest v0.131.1-0.20250801020258-8b73477b9810
	go.opentelemetry.io/collector/filter v0.131.1-0.20250801020258-8b73477b9810
	go.opentelemetry.io/collector/pdata v1.37.1-0.20250801020258-8b73477b9810
	go.opentelemetry.io/collector/receiver v1.37.1-0.20250801020258-8b73477b9810
	go.opentelemetry.io/collector/receiver/receivertest v0.131.1-0.20250801020258-8b73477b9810
	go.opentelemetry.io/collector/scraper v0.131.1-0.20250801020258-8b73477b9810
	go.opentelemetry.io/collector/scraper/scraperhelper v0.131.1-0.20250801020258-8b73477b9810
	go.opentelemetry.io/otel/trace v1.37.0
	go.uber.org/goleak v1.3.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.0
)

require (
	github.com/DataDog/datadog-agent/pkg/util/log v0.68.0 // indirect
	github.com/DataDog/datadog-agent/pkg/util/scrubber v0.68.0 // indirect
	github.com/DataDog/datadog-agent/pkg/version v0.68.0 // indirect
	github.com/DataDog/datadog-go/v5 v5.6.0 // indirect
	github.com/DataDog/go-sqllexer v0.1.6 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cihub/seelog v0.0.0-20170130134532-f561c5e57575 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.2.2 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.131.0 // indirect
	github.com/outcaste-io/ristretto v0.2.3 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.131.1-0.20250801020258-8b73477b9810 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.131.1-0.20250801020258-8b73477b9810 // indirect
	go.opentelemetry.io/collector/featuregate v1.37.1-0.20250801020258-8b73477b9810 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.131.1-0.20250801020258-8b73477b9810 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.131.1-0.20250801020258-8b73477b9810 // indirect
	go.opentelemetry.io/collector/pipeline v0.131.1-0.20250801020258-8b73477b9810 // indirect
	go.opentelemetry.io/collector/receiver/receiverhelper v0.131.1-0.20250801020258-8b73477b9810 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.131.1-0.20250801020258-8b73477b9810 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.12.0 // indirect
	go.opentelemetry.io/otel v1.37.0 // indirect
	go.opentelemetry.io/otel/log v0.13.0 // indirect
	go.opentelemetry.io/otel/metric v1.37.0 // indirect
	go.opentelemetry.io/otel/sdk v1.37.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.37.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/net v0.41.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.26.0 // indirect
	golang.org/x/time v0.11.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250603155806-513f23925822 // indirect
	google.golang.org/grpc v1.74.2 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest
