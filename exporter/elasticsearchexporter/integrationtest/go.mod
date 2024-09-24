module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter/integrationtest

go 1.22.0

require (
	github.com/elastic/go-docappender/v2 v2.3.0
	github.com/gorilla/mux v1.8.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter v0.110.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage v0.110.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.110.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent v0.110.0
	github.com/open-telemetry/opentelemetry-collector-contrib/testbed v0.110.0
	github.com/shirou/gopsutil/v4 v4.24.8
	github.com/stretchr/testify v1.9.0
	go.opentelemetry.io/collector/component v0.110.0
	go.opentelemetry.io/collector/component/componentstatus v0.110.0
	go.opentelemetry.io/collector/config/confighttp v0.110.0
	go.opentelemetry.io/collector/confmap v1.16.0
	go.opentelemetry.io/collector/confmap/provider/fileprovider v1.16.0
	go.opentelemetry.io/collector/consumer v0.110.0
	go.opentelemetry.io/collector/exporter v0.110.0
	go.opentelemetry.io/collector/exporter/debugexporter v0.110.0
	go.opentelemetry.io/collector/extension v0.110.0
	go.opentelemetry.io/collector/otelcol v0.110.0
	go.opentelemetry.io/collector/pdata v1.16.0
	go.opentelemetry.io/collector/processor v0.110.0
	go.opentelemetry.io/collector/receiver v0.110.0
	go.opentelemetry.io/collector/receiver/otlpreceiver v0.110.0
	go.uber.org/zap v1.27.0
	golang.org/x/sync v0.8.0
)

require (
	github.com/alecthomas/participle/v2 v2.1.1 // indirect
	github.com/apache/thrift v0.20.0 // indirect
	github.com/armon/go-radix v1.0.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/census-instrumentation/opencensus-proto v0.4.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/elastic/elastic-transport-go/v8 v8.6.0 // indirect
	github.com/elastic/go-elasticsearch/v7 v7.17.10 // indirect
	github.com/elastic/go-elasticsearch/v8 v8.14.0 // indirect
	github.com/elastic/go-grok v0.3.1 // indirect
	github.com/elastic/go-structform v0.0.12 // indirect
	github.com/elastic/go-sysinfo v1.14.0 // indirect
	github.com/elastic/go-windows v1.0.1 // indirect
	github.com/elastic/lunes v0.1.0 // indirect
	github.com/expr-lang/expr v1.16.9 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.1.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/goccy/go-json v0.10.3 // indirect
	github.com/gogo/googleapis v1.4.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.22.0 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/iancoleman/strcase v0.3.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jaegertracing/jaeger v1.60.0 // indirect
	github.com/jonboulle/clockwork v0.4.0 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/knadh/koanf/maps v0.1.1 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/knadh/koanf/v2 v2.1.1 // indirect
	github.com/leodido/go-syslog/v4 v4.1.0 // indirect
	github.com/leodido/ragel-machinery v0.0.0-20190525184631-5f46317e436b // indirect
	github.com/lestrrat-go/strftime v1.1.0 // indirect
	github.com/lightstep/go-expohisto v1.0.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20240226150601-1dcf7310316a // indirect
	github.com/magefile/mage v1.15.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mostynb/go-grpc-compression v1.2.3 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector v0.110.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector v0.110.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opencensusexporter v0.110.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/syslogexporter v0.110.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/zipkinexporter v0.110.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.110.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil v0.110.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden v0.110.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl v0.110.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.110.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza v0.110.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger v0.110.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus v0.110.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin v0.110.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver v0.110.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/opencensusreceiver v0.110.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/syslogreceiver v0.110.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver v0.110.0 // indirect
	github.com/openzipkin/zipkin-go v0.4.3 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/prometheus/client_golang v1.20.4 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.59.1 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/shoenig/test v1.7.1 // indirect
	github.com/soheilhy/cmux v0.1.5 // indirect
	github.com/spf13/cobra v1.8.1 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/tklauser/go-sysconf v0.3.13 // indirect
	github.com/tklauser/numcpus v0.7.0 // indirect
	github.com/ua-parser/uap-go v0.0.0-20240611065828-3a4781585db6 // indirect
	github.com/valyala/fastjson v1.6.4 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.elastic.co/apm/module/apmelasticsearch/v2 v2.6.0 // indirect
	go.elastic.co/apm/module/apmhttp/v2 v2.6.0 // indirect
	go.elastic.co/apm/module/apmzap/v2 v2.6.0 // indirect
	go.elastic.co/apm/v2 v2.6.0 // indirect
	go.elastic.co/fastjson v1.3.0 // indirect
	go.etcd.io/bbolt v1.3.11 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/collector v0.110.0 // indirect
	go.opentelemetry.io/collector/client v1.16.0 // indirect
	go.opentelemetry.io/collector/component/componentprofiles v0.110.0 // indirect
	go.opentelemetry.io/collector/config/configauth v0.110.0 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.16.0 // indirect
	go.opentelemetry.io/collector/config/configgrpc v0.110.0 // indirect
	go.opentelemetry.io/collector/config/confignet v1.16.0 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.16.0 // indirect
	go.opentelemetry.io/collector/config/configretry v1.16.0 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.110.0 // indirect
	go.opentelemetry.io/collector/config/configtls v1.16.0 // indirect
	go.opentelemetry.io/collector/config/internal v0.110.0 // indirect
	go.opentelemetry.io/collector/connector v0.110.0 // indirect
	go.opentelemetry.io/collector/connector/connectorprofiles v0.110.0 // indirect
	go.opentelemetry.io/collector/consumer/consumerprofiles v0.110.0 // indirect
	go.opentelemetry.io/collector/consumer/consumertest v0.110.0 // indirect
	go.opentelemetry.io/collector/exporter/exporterprofiles v0.110.0 // indirect
	go.opentelemetry.io/collector/exporter/otlpexporter v0.110.0 // indirect
	go.opentelemetry.io/collector/exporter/otlphttpexporter v0.110.0 // indirect
	go.opentelemetry.io/collector/extension/auth v0.110.0 // indirect
	go.opentelemetry.io/collector/extension/experimental/storage v0.110.0 // indirect
	go.opentelemetry.io/collector/extension/extensioncapabilities v0.110.0 // indirect
	go.opentelemetry.io/collector/extension/zpagesextension v0.110.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.16.0 // indirect
	go.opentelemetry.io/collector/internal/globalgates v0.110.0 // indirect
	go.opentelemetry.io/collector/internal/globalsignal v0.110.0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.110.0 // indirect
	go.opentelemetry.io/collector/pdata/testdata v0.110.0 // indirect
	go.opentelemetry.io/collector/pipeline v0.110.0 // indirect
	go.opentelemetry.io/collector/processor/batchprocessor v0.110.0 // indirect
	go.opentelemetry.io/collector/processor/memorylimiterprocessor v0.110.0 // indirect
	go.opentelemetry.io/collector/processor/processorprofiles v0.110.0 // indirect
	go.opentelemetry.io/collector/receiver/receiverprofiles v0.110.0 // indirect
	go.opentelemetry.io/collector/semconv v0.110.0 // indirect
	go.opentelemetry.io/collector/service v0.110.0 // indirect
	go.opentelemetry.io/contrib/config v0.10.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.55.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.55.0 // indirect
	go.opentelemetry.io/contrib/propagators/b3 v1.30.0 // indirect
	go.opentelemetry.io/contrib/zpages v0.55.0 // indirect
	go.opentelemetry.io/otel v1.30.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.6.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.30.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.30.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.30.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.30.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.30.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.52.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutlog v0.6.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v1.30.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.30.0 // indirect
	go.opentelemetry.io/otel/log v0.6.0 // indirect
	go.opentelemetry.io/otel/metric v1.30.0 // indirect
	go.opentelemetry.io/otel/sdk v1.30.0 // indirect
	go.opentelemetry.io/otel/sdk/log v0.6.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.30.0 // indirect
	go.opentelemetry.io/otel/trace v1.30.0 // indirect
	go.opentelemetry.io/proto/otlp v1.3.1 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/exp v0.0.0-20240506185415-9bf2ced13842 // indirect
	golang.org/x/net v0.29.0 // indirect
	golang.org/x/sys v0.25.0 // indirect
	golang.org/x/text v0.18.0 // indirect
	gonum.org/v1/gonum v0.15.1 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240903143218-8af14fe29dc1 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240903143218-8af14fe29dc1 // indirect
	google.golang.org/grpc v1.66.2 // indirect
	google.golang.org/protobuf v1.34.2 // indirect
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
