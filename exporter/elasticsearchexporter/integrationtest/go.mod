module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/integrationtest

go 1.23.0

require (
	github.com/elastic/go-docappender/v2 v2.6.0
	github.com/gorilla/mux v1.8.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter v0.120.1
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage v0.120.1
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.120.1
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent v0.120.1
	github.com/open-telemetry/opentelemetry-collector-contrib/testbed v0.0.0-00010101000000-000000000000
	github.com/shirou/gopsutil/v4 v4.25.1
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v0.120.1-0.20250226024140-8099e51f9a77
	go.opentelemetry.io/collector/component/componentstatus v0.120.1-0.20250226024140-8099e51f9a77
	go.opentelemetry.io/collector/component/componenttest v0.120.1-0.20250226024140-8099e51f9a77
	go.opentelemetry.io/collector/config/confighttp v0.120.1-0.20250226024140-8099e51f9a77
	go.opentelemetry.io/collector/confmap v1.26.1-0.20250226024140-8099e51f9a77
	go.opentelemetry.io/collector/confmap/provider/fileprovider v1.26.1-0.20250226024140-8099e51f9a77
	go.opentelemetry.io/collector/consumer v1.26.1-0.20250226024140-8099e51f9a77
	go.opentelemetry.io/collector/exporter v0.120.1-0.20250226024140-8099e51f9a77
	go.opentelemetry.io/collector/exporter/debugexporter v0.120.1-0.20250226024140-8099e51f9a77
	go.opentelemetry.io/collector/exporter/exportertest v0.120.1-0.20250226024140-8099e51f9a77
	go.opentelemetry.io/collector/extension v0.120.1-0.20250226024140-8099e51f9a77
	go.opentelemetry.io/collector/otelcol v0.120.1-0.20250226024140-8099e51f9a77
	go.opentelemetry.io/collector/pdata v1.26.1-0.20250226024140-8099e51f9a77
	go.opentelemetry.io/collector/processor v0.120.1-0.20250226024140-8099e51f9a77
	go.opentelemetry.io/collector/receiver v0.120.1-0.20250226024140-8099e51f9a77
	go.opentelemetry.io/collector/receiver/otlpreceiver v0.120.1-0.20250226024140-8099e51f9a77
	go.opentelemetry.io/collector/receiver/receivertest v0.120.1-0.20250226024140-8099e51f9a77
	go.uber.org/zap v1.27.0
	golang.org/x/sync v0.11.0
)

require (
	github.com/alecthomas/participle/v2 v2.1.1 // indirect
	github.com/antchfx/xmlquery v1.4.3 // indirect
	github.com/antchfx/xpath v1.3.3 // indirect
	github.com/apache/thrift v0.21.0 // indirect
	github.com/armon/go-radix v1.0.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/census-instrumentation/opencensus-proto v0.4.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cilium/ebpf v0.16.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/ebitengine/purego v0.8.2 // indirect
	github.com/elastic/elastic-transport-go/v8 v8.6.1 // indirect
	github.com/elastic/go-elasticsearch/v8 v8.17.1 // indirect
	github.com/elastic/go-freelru v0.16.0 // indirect
	github.com/elastic/go-grok v0.3.1 // indirect
	github.com/elastic/go-structform v0.0.12 // indirect
	github.com/elastic/go-sysinfo v1.15.0 // indirect
	github.com/elastic/go-windows v1.0.2 // indirect
	github.com/elastic/lunes v0.1.0 // indirect
	github.com/expr-lang/expr v1.16.9 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/gogo/googleapis v1.4.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.26.1 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/iancoleman/strcase v0.3.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jaegertracing/jaeger v1.66.0 // indirect
	github.com/jaegertracing/jaeger-idl v0.5.0 // indirect
	github.com/jonboulle/clockwork v0.4.0 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/klauspost/cpuid/v2 v2.2.8 // indirect
	github.com/knadh/koanf/maps v0.1.1 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/knadh/koanf/v2 v2.1.2 // indirect
	github.com/leodido/go-syslog/v4 v4.2.0 // indirect
	github.com/leodido/ragel-machinery v0.0.0-20190525184631-5f46317e436b // indirect
	github.com/lestrrat-go/strftime v1.1.0 // indirect
	github.com/lightstep/go-expohisto v1.0.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20240226150601-1dcf7310316a // indirect
	github.com/magefile/mage v1.15.0 // indirect
	github.com/minio/sha256-simd v1.0.1 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mostynb/go-grpc-compression v1.2.3 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector v0.120.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector v0.120.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opencensusexporter v0.120.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/syslogexporter v0.120.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/zipkinexporter v0.120.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.120.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil v0.120.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/core/xidutils v0.120.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden v0.120.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl v0.120.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.120.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza v0.120.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger v0.120.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus v0.120.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin v0.120.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver v0.120.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/opencensusreceiver v0.120.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/syslogreceiver v0.120.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver v0.120.1 // indirect
	github.com/openzipkin/zipkin-go v0.4.3 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/prometheus/client_golang v1.20.5 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.62.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/soheilhy/cmux v0.1.5 // indirect
	github.com/spf13/cobra v1.9.1 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/tklauser/go-sysconf v0.3.14 // indirect
	github.com/tklauser/numcpus v0.8.0 // indirect
	github.com/ua-parser/uap-go v0.0.0-20240611065828-3a4781585db6 // indirect
	github.com/valyala/fastjson v1.6.4 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	github.com/zeebo/xxh3 v1.0.2 // indirect
	go.elastic.co/apm/module/apmelasticsearch/v2 v2.6.3 // indirect
	go.elastic.co/apm/module/apmhttp/v2 v2.6.3 // indirect
	go.elastic.co/apm/module/apmzap/v2 v2.6.3 // indirect
	go.elastic.co/apm/v2 v2.6.3 // indirect
	go.elastic.co/fastjson v1.4.0 // indirect
	go.etcd.io/bbolt v1.4.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/client v1.26.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/config/configauth v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.26.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/config/configgrpc v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/config/confignet v1.26.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.26.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/config/configretry v1.26.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/config/configtls v1.26.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/confmap/xconfmap v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/connector v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/connector/connectortest v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/connector/xconnector v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/consumer/consumererror/xconsumererror v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/consumer/consumertest v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/exporter/exporterhelper/xexporterhelper v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/exporter/otlpexporter v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/exporter/otlphttpexporter v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/exporter/xexporter v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/extension/extensionauth v0.120.0 // indirect
	go.opentelemetry.io/collector/extension/extensioncapabilities v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/extension/extensiontest v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/extension/xextension v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/extension/zpagesextension v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/featuregate v1.26.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/internal/fanoutconsumer v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/internal/memorylimiter v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/internal/sharedcomponent v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/pdata/testdata v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/pipeline v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/processor/batchprocessor v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/processor/memorylimiterprocessor v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/processor/processortest v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/processor/xprocessor v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/semconv v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/service v0.120.1-0.20250226024140-8099e51f9a77 // indirect
	go.opentelemetry.io/collector/service/hostcapabilities v0.120.0 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.9.0 // indirect
	go.opentelemetry.io/contrib/config v0.14.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.59.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.59.0 // indirect
	go.opentelemetry.io/contrib/propagators/b3 v1.34.0 // indirect
	go.opentelemetry.io/contrib/zpages v0.59.0 // indirect
	go.opentelemetry.io/ebpf-profiler v0.0.0-20250212075250-7bf12d3f962f // indirect
	go.opentelemetry.io/otel v1.34.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc v0.10.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.10.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.34.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.34.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.34.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.34.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.34.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.56.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutlog v0.10.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v1.34.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.34.0 // indirect
	go.opentelemetry.io/otel/log v0.10.0 // indirect
	go.opentelemetry.io/otel/metric v1.34.0 // indirect
	go.opentelemetry.io/otel/sdk v1.34.0 // indirect
	go.opentelemetry.io/otel/sdk/log v0.10.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.34.0 // indirect
	go.opentelemetry.io/otel/trace v1.34.0 // indirect
	go.opentelemetry.io/proto/otlp v1.5.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/exp v0.0.0-20250210185358-939b2ce775ac // indirect
	golang.org/x/net v0.35.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	gonum.org/v1/gonum v0.15.1 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250204164813-702378808489 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250212204824-5a70512c5d8b // indirect
	google.golang.org/grpc v1.70.0 // indirect
	google.golang.org/protobuf v1.36.5 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	howett.net/plist v1.0.1 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter => ../

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage => ../../../extension/storage

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage => ../../../extension/storage/filestorage

replace github.com/open-telemetry/opentelemetry-collector-contrib/testbed => ../../../testbed

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opencensusexporter => ../../opencensusexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/syslogexporter => ../../syslogexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/zipkinexporter => ../../zipkinexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../../internal/common

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/core/xidutils => ../../../pkg/core/xidutils

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent => ../../../internal/sharedcomponent

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../../pkg/golden

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza => ../../../pkg/stanza

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger => ../../../pkg/translator/jaeger

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus => ../../../pkg/translator/opencensus

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin => ../../../pkg/translator/zipkin

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver => ../../../receiver/jaegerreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/opencensusreceiver => ../../../receiver/opencensusreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/syslogreceiver => ../../../receiver/syslogreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver => ../../../receiver/zipkinreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver => ../../../receiver/signalfxreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata => ../../../pkg/experimentalmetricmetadata

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver => ../../../receiver/prometheusreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver => ../../../receiver/carbonreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/testbed/mockdatasenders/mockdatadogagentexporter => ../../../testbed/mockdatasenders/mockdatadogagentexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite => ../../../pkg/translator/prometheusremotewrite

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk => ../../../internal/splunk

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter => ../../signalfxexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr => ../../../pkg/batchperresourceattr

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver => ../../../receiver/datadogreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter => ../../sapmexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/carbonexporter => ../../carbonexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter => ../../splunkhecexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter => ../../prometheusexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/signalfx => ../../../pkg/translator/signalfx

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver => ../../../receiver/splunkhecreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus => ../../../pkg/translator/prometheus

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver => ../../../receiver/sapmreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry => ../../../pkg/resourcetotelemetry

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter => ../../prometheusremotewriteexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/ackextension => ../../../extension/ackextension

replace github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector => ../../../connector/spanmetricsconnector

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl => ../../../pkg/ottl

replace github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector => ../../../connector/routingconnector

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics => ../../../internal/exp/metrics

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil => ../../../internal/pdatautil

replace go.opentelemetry.io/collector/extension/extensionauth => go.opentelemetry.io/collector/extension/extensionauth v0.0.0-20250226024140-8099e51f9a77

replace go.opentelemetry.io/collector/extension/extensionauth/extensionauthtest => go.opentelemetry.io/collector/extension/extensionauth/extensionauthtest v0.0.0-20250226024140-8099e51f9a77

replace go.opentelemetry.io/collector/service/hostcapabilities => go.opentelemetry.io/collector/service/hostcapabilities v0.0.0-20250226024140-8099e51f9a77
