module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver

go 1.23.0

require (
	github.com/Azure/azure-amqp-common-go/v4 v4.2.0
	github.com/Azure/azure-event-hubs-go/v3 v3.6.2
	github.com/json-iterator/go v1.1.12
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent v0.125.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza v0.125.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azure v0.125.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azurelogs v0.125.0
	github.com/relvacode/iso8601 v1.6.0
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v1.31.1-0.20250505152726-56c7da210783
	go.opentelemetry.io/collector/component/componenttest v0.125.1-0.20250505155216-829157cef7bb
	go.opentelemetry.io/collector/confmap v1.31.1-0.20250505152726-56c7da210783
	go.opentelemetry.io/collector/confmap/xconfmap v0.125.1-0.20250505155216-829157cef7bb
	go.opentelemetry.io/collector/consumer v1.31.1-0.20250505152726-56c7da210783
	go.opentelemetry.io/collector/consumer/consumertest v0.125.1-0.20250505155216-829157cef7bb
	go.opentelemetry.io/collector/extension/xextension v0.125.1-0.20250505155216-829157cef7bb
	go.opentelemetry.io/collector/otelcol/otelcoltest v0.125.1-0.20250505155216-829157cef7bb
	go.opentelemetry.io/collector/pdata v1.31.1-0.20250505152726-56c7da210783
	go.opentelemetry.io/collector/pipeline v0.125.1-0.20250505155216-829157cef7bb
	go.opentelemetry.io/collector/receiver v1.31.1-0.20250505152726-56c7da210783
	go.opentelemetry.io/collector/receiver/receiverhelper v0.125.1-0.20250505155216-829157cef7bb
	go.opentelemetry.io/collector/receiver/receivertest v0.125.1-0.20250505155216-829157cef7bb
	go.opentelemetry.io/collector/semconv v0.125.1-0.20250505155216-829157cef7bb
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.0
)

require (
	github.com/Azure/azure-sdk-for-go v68.0.0+incompatible // indirect
	github.com/Azure/go-amqp v1.0.2 // indirect
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest v0.11.28 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.21 // indirect
	github.com/Azure/go-autorest/autorest/date v0.3.0 // indirect
	github.com/Azure/go-autorest/autorest/to v0.4.0 // indirect
	github.com/Azure/go-autorest/autorest/validation v0.3.1 // indirect
	github.com/Azure/go-autorest/logger v0.2.1 // indirect
	github.com/Azure/go-autorest/tracing v0.6.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/devigned/tab v0.1.1 // indirect
	github.com/ebitengine/purego v0.8.2 // indirect
	github.com/elastic/lunes v0.1.0 // indirect
	github.com/expr-lang/expr v1.17.2 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.26.1 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.2.0 // indirect
	github.com/leodido/go-syslog/v4 v4.2.0 // indirect
	github.com/leodido/ragel-machinery v0.0.0-20190525184631-5f46317e436b // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/magefile/mage v1.15.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.1-0.20231216201459-8508981c8b6c // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.125.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/prometheus/client_golang v1.21.1 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.62.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/shirou/gopsutil/v4 v4.25.3 // indirect
	github.com/spf13/cobra v1.9.1 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/valyala/fastjson v1.6.4 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/component/componentstatus v0.125.1-0.20250505155216-829157cef7bb // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.125.1-0.20250505155216-829157cef7bb // indirect
	go.opentelemetry.io/collector/confmap/provider/envprovider v1.31.1-0.20250505152726-56c7da210783 // indirect
	go.opentelemetry.io/collector/confmap/provider/fileprovider v1.31.1-0.20250505152726-56c7da210783 // indirect
	go.opentelemetry.io/collector/confmap/provider/httpprovider v1.31.1-0.20250505152726-56c7da210783 // indirect
	go.opentelemetry.io/collector/confmap/provider/yamlprovider v1.31.1-0.20250505152726-56c7da210783 // indirect
	go.opentelemetry.io/collector/connector v0.125.1-0.20250505155216-829157cef7bb // indirect
	go.opentelemetry.io/collector/connector/connectortest v0.125.1-0.20250505155216-829157cef7bb // indirect
	go.opentelemetry.io/collector/connector/xconnector v0.125.1-0.20250505155216-829157cef7bb // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.125.1-0.20250505155216-829157cef7bb // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.125.1-0.20250505155216-829157cef7bb // indirect
	go.opentelemetry.io/collector/exporter v0.125.1-0.20250505155216-829157cef7bb // indirect
	go.opentelemetry.io/collector/exporter/exportertest v0.125.1-0.20250505155216-829157cef7bb // indirect
	go.opentelemetry.io/collector/exporter/xexporter v0.125.1-0.20250505155216-829157cef7bb // indirect
	go.opentelemetry.io/collector/extension v1.31.1-0.20250505152726-56c7da210783 // indirect
	go.opentelemetry.io/collector/extension/extensioncapabilities v0.125.1-0.20250505155216-829157cef7bb // indirect
	go.opentelemetry.io/collector/extension/extensiontest v0.125.1-0.20250505155216-829157cef7bb // indirect
	go.opentelemetry.io/collector/featuregate v1.31.1-0.20250505152726-56c7da210783 // indirect
	go.opentelemetry.io/collector/internal/fanoutconsumer v0.125.1-0.20250505155216-829157cef7bb // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.125.1-0.20250505155216-829157cef7bb // indirect
	go.opentelemetry.io/collector/otelcol v0.125.1-0.20250505155216-829157cef7bb // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.125.1-0.20250505155216-829157cef7bb // indirect
	go.opentelemetry.io/collector/pdata/testdata v0.125.1-0.20250505155216-829157cef7bb // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.125.1-0.20250505155216-829157cef7bb // indirect
	go.opentelemetry.io/collector/processor v1.31.1-0.20250505152726-56c7da210783 // indirect
	go.opentelemetry.io/collector/processor/processortest v0.125.1-0.20250505155216-829157cef7bb // indirect
	go.opentelemetry.io/collector/processor/xprocessor v0.125.1-0.20250505155216-829157cef7bb // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.125.1-0.20250505155216-829157cef7bb // indirect
	go.opentelemetry.io/collector/service v0.125.1-0.20250505155216-829157cef7bb // indirect
	go.opentelemetry.io/collector/service/hostcapabilities v0.125.1-0.20250505155216-829157cef7bb // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.10.0 // indirect
	go.opentelemetry.io/contrib/otelconf v0.15.0 // indirect
	go.opentelemetry.io/contrib/propagators/b3 v1.35.0 // indirect
	go.opentelemetry.io/otel v1.35.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc v0.11.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.11.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.35.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.35.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.35.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.35.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.35.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.57.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutlog v0.11.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v1.35.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.35.0 // indirect
	go.opentelemetry.io/otel/log v0.11.0 // indirect
	go.opentelemetry.io/otel/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk/log v0.11.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/trace v1.35.0 // indirect
	go.opentelemetry.io/proto/otlp v1.5.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/exp v0.0.0-20240506185415-9bf2ced13842 // indirect
	golang.org/x/net v0.39.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.25.0 // indirect
	gonum.org/v1/gonum v0.16.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250218202821-56aae31c358a // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250218202821-56aae31c358a // indirect
	google.golang.org/grpc v1.72.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
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
