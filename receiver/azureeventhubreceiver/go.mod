module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver

go 1.26.0

require (
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.23.1
	github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/v2 v2.0.2
	github.com/Azure/azure-sdk-for-go/sdk/storage/azblob v1.8.0
	github.com/goccy/go-json v0.10.6
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent v0.160.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza v0.160.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azure v0.160.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azurelogs v0.160.0
	github.com/relvacode/iso8601 v1.8.0
	github.com/stretchr/testify v1.12.1
	go.opentelemetry.io/collector/component v1.66.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/component/componenttest v0.160.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/confmap v1.66.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/consumer v1.66.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/consumer/consumertest v0.160.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/extension/xextension v0.160.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/pdata v1.66.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/pipeline v1.66.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/receiver v1.66.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/receiver/receiverhelper v0.160.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/receiver/receivertest v0.160.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/collector/receiver/xreceiver v0.160.1-0.20260903163450-cc4b33fc673f
	go.opentelemetry.io/otel v1.46.0
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.28.0
)

require (
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.14.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.12.0 // indirect
	github.com/Azure/go-amqp v1.5.0 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/elastic/lunes v0.2.2 // indirect
	github.com/expr-lang/expr v1.17.8 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.3 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.1 // indirect
	github.com/knadh/koanf/v2 v2.3.6 // indirect
	github.com/leodido/go-syslog/v4 v4.6.0 // indirect
	github.com/leodido/ragel-machinery v0.0.0-20190525184631-5f46317e436b // indirect
	github.com/magefile/mage v1.15.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.160.0 // indirect
	github.com/valyala/fastjson v1.6.10 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.160.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.160.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/extension v1.66.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/featuregate v1.66.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.160.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.160.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.160.1-0.20260903163450-cc4b33fc673f // indirect
	go.opentelemetry.io/otel/metric v1.46.0 // indirect
	go.opentelemetry.io/otel/sdk v1.46.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.46.0 // indirect
	go.opentelemetry.io/otel/trace v1.46.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.41.0 // indirect
	gonum.org/v1/gonum v0.17.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260610212136-7ab31c22f7ad // indirect
	google.golang.org/grpc v1.83.2 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
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
