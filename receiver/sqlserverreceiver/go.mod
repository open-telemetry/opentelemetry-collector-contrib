module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver

go 1.25.0

require (
	github.com/DataDog/datadog-agent/pkg/obfuscate v0.76.0-devel
	github.com/google/go-cmp v0.7.0
	github.com/hashicorp/golang-lru/v2 v2.0.7
	github.com/microsoft/go-mssqldb v1.9.6
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.144.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery v0.144.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden v0.144.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.144.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters v0.144.0
	github.com/stretchr/testify v1.11.1
	github.com/testcontainers/testcontainers-go v0.40.0
	go.opentelemetry.io/collector/component v1.51.0
	go.opentelemetry.io/collector/component/componenttest v0.145.0
	go.opentelemetry.io/collector/config/configopaque v1.50.0
	go.opentelemetry.io/collector/confmap v1.51.0
	go.opentelemetry.io/collector/confmap/xconfmap v0.145.0
	go.opentelemetry.io/collector/consumer v1.51.0
	go.opentelemetry.io/collector/consumer/consumertest v0.145.0
	go.opentelemetry.io/collector/featuregate v1.51.0
	go.opentelemetry.io/collector/filter v0.144.0
	go.opentelemetry.io/collector/pdata v1.51.0
	go.opentelemetry.io/collector/receiver v1.51.0
	go.opentelemetry.io/collector/receiver/receivertest v0.145.0
	go.opentelemetry.io/collector/scraper v0.145.0
	go.opentelemetry.io/collector/scraper/scraperhelper v0.144.0
	go.opentelemetry.io/otel v1.40.0
	go.opentelemetry.io/otel/trace v1.40.0
	go.uber.org/goleak v1.3.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	dario.cat/mergo v1.0.2 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/DataDog/datadog-go/v5 v5.8.2 // indirect
	github.com/DataDog/go-sqllexer v0.1.10 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/platforms v0.2.1 // indirect
	github.com/cpuguy83/dockercfg v0.3.2 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/docker v28.5.1+incompatible // indirect
	github.com/docker/go-connections v0.6.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/ebitengine/purego v0.8.4 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/golang-sql/civil v0.0.0-20220223132316-b832511892a9 // indirect
	github.com/golang-sql/sqlexp v0.1.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-uuid v1.0.3 // indirect
	github.com/hashicorp/go-version v1.8.0 // indirect
	github.com/jcmturner/aescts/v2 v2.0.0 // indirect
	github.com/jcmturner/dnsutils/v2 v2.0.0 // indirect
	github.com/jcmturner/gofork v1.7.6 // indirect
	github.com/jcmturner/goidentity/v6 v6.0.1 // indirect
	github.com/jcmturner/gokrb5/v8 v8.4.4 // indirect
	github.com/jcmturner/rpc/v2 v2.0.3 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.2 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/magiconair/properties v1.8.10 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/go-archive v0.1.0 // indirect
	github.com/moby/patternmatcher v0.6.0 // indirect
	github.com/moby/sys/sequential v0.6.0 // indirect
	github.com/moby/sys/user v0.4.0 // indirect
	github.com/moby/sys/userns v0.1.0 // indirect
	github.com/moby/term v0.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.144.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/outcaste-io/ristretto v0.2.3 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/shirou/gopsutil/v4 v4.25.6 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.145.0 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.145.0 // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.145.0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.145.0 // indirect
	go.opentelemetry.io/collector/pipeline v1.51.0 // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.145.0 // indirect
	go.opentelemetry.io/collector/receiver/receiverhelper v0.145.0 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.145.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.49.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.37.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.28.0 // indirect
	go.opentelemetry.io/otel/metric v1.40.0 // indirect
	go.opentelemetry.io/otel/sdk v1.40.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.40.0 // indirect
	go.opentelemetry.io/proto/otlp v1.7.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.46.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	golang.org/x/time v0.12.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251222181119-0a764e51fe1b // indirect
	google.golang.org/grpc v1.78.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters => ../../pkg/winperfcounters

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery => ../../internal/sqlquery

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

replace go.opentelemetry.io/collector => /Users/sambalam/Splunk/repos/opentelemetry-collector

replace go.opentelemetry.io/collector/internal/componentalias => /Users/sambalam/Splunk/repos/opentelemetry-collector/internal/componentalias

replace go.opentelemetry.io/collector/internal/memorylimiter => /Users/sambalam/Splunk/repos/opentelemetry-collector/internal/memorylimiter

replace go.opentelemetry.io/collector/internal/fanoutconsumer => /Users/sambalam/Splunk/repos/opentelemetry-collector/internal/fanoutconsumer

replace go.opentelemetry.io/collector/internal/sharedcomponent => /Users/sambalam/Splunk/repos/opentelemetry-collector/internal/sharedcomponent

replace go.opentelemetry.io/collector/internal/telemetry => /Users/sambalam/Splunk/repos/opentelemetry-collector/internal/telemetry

replace go.opentelemetry.io/collector/internal/testutil => /Users/sambalam/Splunk/repos/opentelemetry-collector/internal/testutil

replace go.opentelemetry.io/collector/cmd/builder => /Users/sambalam/Splunk/repos/opentelemetry-collector/cmd/builder

replace go.opentelemetry.io/collector/cmd/mdatagen => /Users/sambalam/Splunk/repos/opentelemetry-collector/cmd/mdatagen

replace go.opentelemetry.io/collector/component/componentstatus => /Users/sambalam/Splunk/repos/opentelemetry-collector/component/componentstatus

replace go.opentelemetry.io/collector/component/componenttest => /Users/sambalam/Splunk/repos/opentelemetry-collector/component/componenttest

replace go.opentelemetry.io/collector/confmap/xconfmap => /Users/sambalam/Splunk/repos/opentelemetry-collector/confmap/xconfmap

replace go.opentelemetry.io/collector/config/configgrpc => /Users/sambalam/Splunk/repos/opentelemetry-collector/config/configgrpc

replace go.opentelemetry.io/collector/config/confighttp => /Users/sambalam/Splunk/repos/opentelemetry-collector/config/confighttp

replace go.opentelemetry.io/collector/config/confighttp/xconfighttp => /Users/sambalam/Splunk/repos/opentelemetry-collector/config/confighttp/xconfighttp

replace go.opentelemetry.io/collector/config/configtelemetry => /Users/sambalam/Splunk/repos/opentelemetry-collector/config/configtelemetry

replace go.opentelemetry.io/collector/connector => /Users/sambalam/Splunk/repos/opentelemetry-collector/connector

replace go.opentelemetry.io/collector/connector/connectortest => /Users/sambalam/Splunk/repos/opentelemetry-collector/connector/connectortest

replace go.opentelemetry.io/collector/connector/forwardconnector => /Users/sambalam/Splunk/repos/opentelemetry-collector/connector/forwardconnector

replace go.opentelemetry.io/collector/connector/xconnector => /Users/sambalam/Splunk/repos/opentelemetry-collector/connector/xconnector

replace go.opentelemetry.io/collector/consumer/xconsumer => /Users/sambalam/Splunk/repos/opentelemetry-collector/consumer/xconsumer

replace go.opentelemetry.io/collector/consumer/consumererror => /Users/sambalam/Splunk/repos/opentelemetry-collector/consumer/consumererror

replace go.opentelemetry.io/collector/consumer/consumererror/xconsumererror => /Users/sambalam/Splunk/repos/opentelemetry-collector/consumer/consumererror/xconsumererror

replace go.opentelemetry.io/collector/consumer/consumertest => /Users/sambalam/Splunk/repos/opentelemetry-collector/consumer/consumertest

replace go.opentelemetry.io/collector/exporter/debugexporter => /Users/sambalam/Splunk/repos/opentelemetry-collector/exporter/debugexporter

replace go.opentelemetry.io/collector/exporter/exporterhelper => /Users/sambalam/Splunk/repos/opentelemetry-collector/exporter/exporterhelper

replace go.opentelemetry.io/collector/exporter/exporterhelper/xexporterhelper => /Users/sambalam/Splunk/repos/opentelemetry-collector/exporter/exporterhelper/xexporterhelper

replace go.opentelemetry.io/collector/exporter/exportertest => /Users/sambalam/Splunk/repos/opentelemetry-collector/exporter/exportertest

replace go.opentelemetry.io/collector/exporter/nopexporter => /Users/sambalam/Splunk/repos/opentelemetry-collector/exporter/nopexporter

replace go.opentelemetry.io/collector/exporter/otlpexporter => /Users/sambalam/Splunk/repos/opentelemetry-collector/exporter/otlpexporter

replace go.opentelemetry.io/collector/exporter/otlphttpexporter => /Users/sambalam/Splunk/repos/opentelemetry-collector/exporter/otlphttpexporter

replace go.opentelemetry.io/collector/exporter/xexporter => /Users/sambalam/Splunk/repos/opentelemetry-collector/exporter/xexporter

replace go.opentelemetry.io/collector/extension/extensionauth/extensionauthtest => /Users/sambalam/Splunk/repos/opentelemetry-collector/extension/extensionauth/extensionauthtest

replace go.opentelemetry.io/collector/extension/extensioncapabilities => /Users/sambalam/Splunk/repos/opentelemetry-collector/extension/extensioncapabilities

replace go.opentelemetry.io/collector/extension/extensionmiddleware => /Users/sambalam/Splunk/repos/opentelemetry-collector/extension/extensionmiddleware

replace go.opentelemetry.io/collector/extension/extensionmiddleware/extensionmiddlewaretest => /Users/sambalam/Splunk/repos/opentelemetry-collector/extension/extensionmiddleware/extensionmiddlewaretest

replace go.opentelemetry.io/collector/extension/extensiontest => /Users/sambalam/Splunk/repos/opentelemetry-collector/extension/extensiontest

replace go.opentelemetry.io/collector/extension/zpagesextension => /Users/sambalam/Splunk/repos/opentelemetry-collector/extension/zpagesextension

replace go.opentelemetry.io/collector/extension/memorylimiterextension => /Users/sambalam/Splunk/repos/opentelemetry-collector/extension/memorylimiterextension

replace go.opentelemetry.io/collector/extension/xextension => /Users/sambalam/Splunk/repos/opentelemetry-collector/extension/xextension

replace go.opentelemetry.io/collector/otelcol => /Users/sambalam/Splunk/repos/opentelemetry-collector/otelcol

replace go.opentelemetry.io/collector/otelcol/otelcoltest => /Users/sambalam/Splunk/repos/opentelemetry-collector/otelcol/otelcoltest

replace go.opentelemetry.io/collector/pdata/pprofile => /Users/sambalam/Splunk/repos/opentelemetry-collector/pdata/pprofile

replace go.opentelemetry.io/collector/pdata/testdata => /Users/sambalam/Splunk/repos/opentelemetry-collector/pdata/testdata

replace go.opentelemetry.io/collector/pdata/xpdata => /Users/sambalam/Splunk/repos/opentelemetry-collector/pdata/xpdata

replace go.opentelemetry.io/collector/pipeline/xpipeline => /Users/sambalam/Splunk/repos/opentelemetry-collector/pipeline/xpipeline

replace go.opentelemetry.io/collector/processor/processortest => /Users/sambalam/Splunk/repos/opentelemetry-collector/processor/processortest

replace go.opentelemetry.io/collector/processor/processorhelper => /Users/sambalam/Splunk/repos/opentelemetry-collector/processor/processorhelper

replace go.opentelemetry.io/collector/processor/batchprocessor => /Users/sambalam/Splunk/repos/opentelemetry-collector/processor/batchprocessor

replace go.opentelemetry.io/collector/processor/memorylimiterprocessor => /Users/sambalam/Splunk/repos/opentelemetry-collector/processor/memorylimiterprocessor

replace go.opentelemetry.io/collector/processor/processorhelper/xprocessorhelper => /Users/sambalam/Splunk/repos/opentelemetry-collector/processor/processorhelper/xprocessorhelper

replace go.opentelemetry.io/collector/processor/xprocessor => /Users/sambalam/Splunk/repos/opentelemetry-collector/processor/xprocessor

replace go.opentelemetry.io/collector/receiver/receiverhelper => /Users/sambalam/Splunk/repos/opentelemetry-collector/receiver/receiverhelper

replace go.opentelemetry.io/collector/receiver/nopreceiver => /Users/sambalam/Splunk/repos/opentelemetry-collector/receiver/nopreceiver

replace go.opentelemetry.io/collector/receiver/otlpreceiver => /Users/sambalam/Splunk/repos/opentelemetry-collector/receiver/otlpreceiver

replace go.opentelemetry.io/collector/receiver/receivertest => /Users/sambalam/Splunk/repos/opentelemetry-collector/receiver/receivertest

replace go.opentelemetry.io/collector/receiver/xreceiver => /Users/sambalam/Splunk/repos/opentelemetry-collector/receiver/xreceiver

replace go.opentelemetry.io/collector/scraper => /Users/sambalam/Splunk/repos/opentelemetry-collector/scraper

replace go.opentelemetry.io/collector/scraper/scraperhelper => /Users/sambalam/Splunk/repos/opentelemetry-collector/scraper/scraperhelper

replace go.opentelemetry.io/collector/scraper/scraperhelper/xscraperhelper => /Users/sambalam/Splunk/repos/opentelemetry-collector/scraper/scraperhelper/xscraperhelper

replace go.opentelemetry.io/collector/scraper/scrapertest => /Users/sambalam/Splunk/repos/opentelemetry-collector/scraper/scrapertest

replace go.opentelemetry.io/collector/scraper/xscraper => /Users/sambalam/Splunk/repos/opentelemetry-collector/scraper/xscraper

replace go.opentelemetry.io/collector/service => /Users/sambalam/Splunk/repos/opentelemetry-collector/service

replace go.opentelemetry.io/collector/service/hostcapabilities => /Users/sambalam/Splunk/repos/opentelemetry-collector/service/hostcapabilities

replace go.opentelemetry.io/collector/service/telemetry/telemetrytest => /Users/sambalam/Splunk/repos/opentelemetry-collector/service/telemetry/telemetrytest

replace go.opentelemetry.io/collector/filter => /Users/sambalam/Splunk/repos/opentelemetry-collector/filter

replace go.opentelemetry.io/collector/client => /Users/sambalam/Splunk/repos/opentelemetry-collector/client

replace go.opentelemetry.io/collector/featuregate => /Users/sambalam/Splunk/repos/opentelemetry-collector/featuregate

replace go.opentelemetry.io/collector/pdata => /Users/sambalam/Splunk/repos/opentelemetry-collector/pdata

replace go.opentelemetry.io/collector/component => /Users/sambalam/Splunk/repos/opentelemetry-collector/component

replace go.opentelemetry.io/collector/confmap => /Users/sambalam/Splunk/repos/opentelemetry-collector/confmap

replace go.opentelemetry.io/collector/confmap/provider/envprovider => /Users/sambalam/Splunk/repos/opentelemetry-collector/confmap/provider/envprovider

replace go.opentelemetry.io/collector/confmap/provider/fileprovider => /Users/sambalam/Splunk/repos/opentelemetry-collector/confmap/provider/fileprovider

replace go.opentelemetry.io/collector/confmap/provider/httpprovider => /Users/sambalam/Splunk/repos/opentelemetry-collector/confmap/provider/httpprovider

replace go.opentelemetry.io/collector/confmap/provider/httpsprovider => /Users/sambalam/Splunk/repos/opentelemetry-collector/confmap/provider/httpsprovider

replace go.opentelemetry.io/collector/confmap/provider/yamlprovider => /Users/sambalam/Splunk/repos/opentelemetry-collector/confmap/provider/yamlprovider

replace go.opentelemetry.io/collector/config/configauth => /Users/sambalam/Splunk/repos/opentelemetry-collector/config/configauth

replace go.opentelemetry.io/collector/config/configopaque => /Users/sambalam/Splunk/repos/opentelemetry-collector/config/configopaque

replace go.opentelemetry.io/collector/config/configoptional => /Users/sambalam/Splunk/repos/opentelemetry-collector/config/configoptional

replace go.opentelemetry.io/collector/config/configcompression => /Users/sambalam/Splunk/repos/opentelemetry-collector/config/configcompression

replace go.opentelemetry.io/collector/config/configretry => /Users/sambalam/Splunk/repos/opentelemetry-collector/config/configretry

replace go.opentelemetry.io/collector/config/configtls => /Users/sambalam/Splunk/repos/opentelemetry-collector/config/configtls

replace go.opentelemetry.io/collector/config/confignet => /Users/sambalam/Splunk/repos/opentelemetry-collector/config/confignet

replace go.opentelemetry.io/collector/config/configmiddleware => /Users/sambalam/Splunk/repos/opentelemetry-collector/config/configmiddleware

replace go.opentelemetry.io/collector/consumer => /Users/sambalam/Splunk/repos/opentelemetry-collector/consumer

replace go.opentelemetry.io/collector/exporter => /Users/sambalam/Splunk/repos/opentelemetry-collector/exporter

replace go.opentelemetry.io/collector/extension => /Users/sambalam/Splunk/repos/opentelemetry-collector/extension

replace go.opentelemetry.io/collector/extension/extensionauth => /Users/sambalam/Splunk/repos/opentelemetry-collector/extension/extensionauth

replace go.opentelemetry.io/collector/pipeline => /Users/sambalam/Splunk/repos/opentelemetry-collector/pipeline

replace go.opentelemetry.io/collector/processor => /Users/sambalam/Splunk/repos/opentelemetry-collector/processor

replace go.opentelemetry.io/collector/receiver => /Users/sambalam/Splunk/repos/opentelemetry-collector/receiver
