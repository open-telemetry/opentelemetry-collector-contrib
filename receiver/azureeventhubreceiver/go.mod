module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver

go 1.25.0

require (
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.21.0
	github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/v2 v2.0.1
	github.com/goccy/go-json v0.10.5
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent v0.146.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza v0.146.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azure v0.146.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azurelogs v0.146.0
	github.com/relvacode/iso8601 v1.7.0
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/component v1.52.0
	go.opentelemetry.io/collector/component/componenttest v0.146.1
	go.opentelemetry.io/collector/confmap v1.52.0
	go.opentelemetry.io/collector/confmap/xconfmap v0.146.1
	go.opentelemetry.io/collector/consumer v1.52.0
	go.opentelemetry.io/collector/consumer/consumertest v0.146.1
	go.opentelemetry.io/collector/extension/xextension v0.146.1
	go.opentelemetry.io/collector/featuregate v1.52.0
	go.opentelemetry.io/collector/pdata v1.52.0
	go.opentelemetry.io/collector/pipeline v1.52.0
	go.opentelemetry.io/collector/receiver v1.52.0
	go.opentelemetry.io/collector/receiver/receiverhelper v0.146.1
	go.opentelemetry.io/collector/receiver/receivertest v0.146.1
	go.opentelemetry.io/collector/receiver/xreceiver v0.146.1
	go.opentelemetry.io/otel v1.40.0
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.1
)

require (
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.11.2 // indirect
	github.com/Azure/go-amqp v1.4.0 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/elastic/lunes v0.2.0 // indirect
	github.com/expr-lang/expr v1.17.8 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.8.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.2 // indirect
	github.com/leodido/go-syslog/v4 v4.3.0 // indirect
	github.com/leodido/ragel-machinery v0.0.0-20190525184631-5f46317e436b // indirect
	github.com/magefile/mage v1.15.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.146.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/valyala/fastjson v1.6.7 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.146.1 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.146.1 // indirect
	go.opentelemetry.io/collector/extension v1.52.0 // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.146.1 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.146.1 // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.146.1 // indirect
	go.opentelemetry.io/otel/metric v1.40.0 // indirect
	go.opentelemetry.io/otel/sdk v1.40.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.40.0 // indirect
	go.opentelemetry.io/otel/trace v1.40.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/exp v0.0.0-20240506185415-9bf2ced13842 // indirect
	golang.org/x/net v0.49.0 // indirect
	golang.org/x/sys v0.41.0 // indirect
	golang.org/x/text v0.34.0 // indirect
	gonum.org/v1/gonum v0.17.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251222181119-0a764e51fe1b // indirect
	google.golang.org/grpc v1.79.1 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage => ../../extension/storage

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza => ../../pkg/stanza

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent => ../../internal/sharedcomponent

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azure => ../../pkg/translator/azure

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azurelogs => ../../pkg/translator/azurelogs

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common
