module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/natscoreexporter

go 1.24.5

require (
	github.com/google/uuid v1.6.0
	github.com/nats-io/nats.go v1.45.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding v0.134.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl v0.133.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.133.0
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/component v1.40.0
	go.opentelemetry.io/collector/component/componenttest v0.133.0
	go.opentelemetry.io/collector/config/configtls v1.39.0
	go.opentelemetry.io/collector/confmap v1.39.0
	go.opentelemetry.io/collector/confmap/xconfmap v0.133.0
	go.opentelemetry.io/collector/exporter v0.133.0
	go.opentelemetry.io/collector/exporter/exportertest v0.133.0
	go.opentelemetry.io/collector/pdata v1.40.0
	go.uber.org/goleak v1.3.0
	go.uber.org/multierr v1.11.0
)

require (
	github.com/alecthomas/participle/v2 v2.1.4 // indirect
	github.com/antchfx/xmlquery v1.4.4 // indirect
	github.com/antchfx/xpath v1.3.5 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/elastic/go-grok v0.3.1 // indirect
	github.com/elastic/lunes v0.1.0 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20250520203025-c3c3a4ec1653 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/google/go-tpm v0.9.5 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/iancoleman/strcase v0.3.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.2.2 // indirect
	github.com/magefile/mage v1.15.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/nats-io/nkeys v0.4.11 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.133.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.133.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/twmb/murmur3 v1.1.8 // indirect
	github.com/ua-parser/uap-go v0.0.0-20250326155420-f7f5a2f9f5bc // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/client v1.39.0 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.39.0 // indirect
	go.opentelemetry.io/collector/config/configoptional v0.133.0 // indirect
	go.opentelemetry.io/collector/config/configretry v1.39.0 // indirect
	go.opentelemetry.io/collector/consumer v1.39.0 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.133.0 // indirect
	go.opentelemetry.io/collector/consumer/consumertest v0.133.0 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.133.0 // indirect
	go.opentelemetry.io/collector/exporter/xexporter v0.133.0 // indirect
	go.opentelemetry.io/collector/extension v1.40.0 // indirect
	go.opentelemetry.io/collector/extension/xextension v0.133.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.40.0 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.134.0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.134.0 // indirect
	go.opentelemetry.io/collector/pdata/xpdata v0.133.0 // indirect
	go.opentelemetry.io/collector/pipeline v1.40.0 // indirect
	go.opentelemetry.io/collector/receiver v1.39.0 // indirect
	go.opentelemetry.io/collector/receiver/receivertest v0.133.0 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.133.0 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.12.0 // indirect
	go.opentelemetry.io/otel v1.37.0 // indirect
	go.opentelemetry.io/otel/log v0.13.0 // indirect
	go.opentelemetry.io/otel/metric v1.37.0 // indirect
	go.opentelemetry.io/otel/sdk v1.37.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.37.0 // indirect
	go.opentelemetry.io/otel/trace v1.37.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.41.0 // indirect
	golang.org/x/exp v0.0.0-20250819193227-8b4c13bb791b // indirect
	golang.org/x/net v0.43.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/text v0.28.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250826171959-ef028d996bc1 // indirect
	google.golang.org/grpc v1.75.0 // indirect
	google.golang.org/protobuf v1.36.8 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
