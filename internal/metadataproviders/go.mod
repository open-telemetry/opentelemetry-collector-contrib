module github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders

go 1.24.0

require (
	github.com/Showmax/go-fqdn v1.0.0
	github.com/aws/aws-sdk-go-v2 v1.39.6
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.13
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.266.0
	github.com/docker/docker v28.5.2+incompatible
	github.com/hashicorp/consul/api v1.32.1
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig v0.139.0
	github.com/shirou/gopsutil/v4 v4.25.10
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/otel v1.38.0
	go.opentelemetry.io/otel/sdk v1.38.0
	go.uber.org/goleak v1.3.0
	k8s.io/api v0.34.1
	k8s.io/apimachinery v0.34.1
	k8s.io/client-go v0.34.1
)

require (
	github.com/Microsoft/go-winio v0.5.2 // indirect
	github.com/armon/go-metrics v0.4.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.13 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.13 // indirect
	github.com/aws/smithy-go v1.23.2 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/distribution/reference v0.5.0 // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/ebitengine/purego v0.9.0 // indirect
	github.com/emicklei/go-restful/v3 v3.12.2 // indirect
	github.com/fatih/color v1.16.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/gnostic-models v0.7.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-hclog v1.5.0 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/hashicorp/serf v0.10.1 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/sys/atomicwriter v0.1.0 // indirect
	github.com/moby/term v0.0.0-20210619224110-3f7ff695adc6 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.2 // indirect
	github.com/openshift/api v0.0.0-20251015095338-264e80a2b6e7 // indirect
	github.com/openshift/client-go v0.0.0-20251015124057-db0dee36e235 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/tklauser/go-sysconf v0.3.15 // indirect
	github.com/tklauser/numcpus v0.10.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.49.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.24.0 // indirect
	go.opentelemetry.io/otel/metric v1.38.0 // indirect
	go.opentelemetry.io/otel/trace v1.38.0 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/exp v0.0.0-20250305212735-054e65f0b394 // indirect
	golang.org/x/net v0.42.0 // indirect
	golang.org/x/oauth2 v0.30.0 // indirect
	golang.org/x/sys v0.37.0 // indirect
	golang.org/x/term v0.33.0 // indirect
	golang.org/x/text v0.27.0 // indirect
	golang.org/x/time v0.9.0 // indirect
	google.golang.org/grpc v1.76.0 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.12.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	gotest.tools/v3 v3.2.0 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/kube-openapi v0.0.0-20250710124328-f3f2b991d03b // indirect
	k8s.io/utils v0.0.0-20250604170112-4c0f3b243397 // indirect
	sigs.k8s.io/json v0.0.0-20241014173422-cfa47c3a1cc8 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.3.0 // indirect
	sigs.k8s.io/yaml v1.6.0 // indirect
)

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig => ../k8sconfig

replace go.opentelemetry.io/collector => /home/jmacd/src/otel/opentelemetry-collector

replace go.opentelemetry.io/collector/internal/memorylimiter => /home/jmacd/src/otel/opentelemetry-collector/internal/memorylimiter

replace go.opentelemetry.io/collector/internal/fanoutconsumer => /home/jmacd/src/otel/opentelemetry-collector/internal/fanoutconsumer

replace go.opentelemetry.io/collector/internal/sharedcomponent => /home/jmacd/src/otel/opentelemetry-collector/internal/sharedcomponent

replace go.opentelemetry.io/collector/internal/telemetry => /home/jmacd/src/otel/opentelemetry-collector/internal/telemetry

replace go.opentelemetry.io/collector/cmd/builder => /home/jmacd/src/otel/opentelemetry-collector/cmd/builder

replace go.opentelemetry.io/collector/cmd/mdatagen => /home/jmacd/src/otel/opentelemetry-collector/cmd/mdatagen

replace go.opentelemetry.io/collector/component/componentstatus => /home/jmacd/src/otel/opentelemetry-collector/component/componentstatus

replace go.opentelemetry.io/collector/component/componenttest => /home/jmacd/src/otel/opentelemetry-collector/component/componenttest

replace go.opentelemetry.io/collector/confmap/xconfmap => /home/jmacd/src/otel/opentelemetry-collector/confmap/xconfmap

replace go.opentelemetry.io/collector/config/configgrpc => /home/jmacd/src/otel/opentelemetry-collector/config/configgrpc

replace go.opentelemetry.io/collector/config/confighttp => /home/jmacd/src/otel/opentelemetry-collector/config/confighttp

replace go.opentelemetry.io/collector/config/confighttp/xconfighttp => /home/jmacd/src/otel/opentelemetry-collector/config/confighttp/xconfighttp

replace go.opentelemetry.io/collector/config/configtelemetry => /home/jmacd/src/otel/opentelemetry-collector/config/configtelemetry

replace go.opentelemetry.io/collector/connector => /home/jmacd/src/otel/opentelemetry-collector/connector

replace go.opentelemetry.io/collector/connector/connectortest => /home/jmacd/src/otel/opentelemetry-collector/connector/connectortest

replace go.opentelemetry.io/collector/connector/forwardconnector => /home/jmacd/src/otel/opentelemetry-collector/connector/forwardconnector

replace go.opentelemetry.io/collector/connector/xconnector => /home/jmacd/src/otel/opentelemetry-collector/connector/xconnector

replace go.opentelemetry.io/collector/consumer/xconsumer => /home/jmacd/src/otel/opentelemetry-collector/consumer/xconsumer

replace go.opentelemetry.io/collector/consumer/consumererror => /home/jmacd/src/otel/opentelemetry-collector/consumer/consumererror

replace go.opentelemetry.io/collector/consumer/consumererror/xconsumererror => /home/jmacd/src/otel/opentelemetry-collector/consumer/consumererror/xconsumererror

replace go.opentelemetry.io/collector/consumer/consumertest => /home/jmacd/src/otel/opentelemetry-collector/consumer/consumertest

replace go.opentelemetry.io/collector/exporter/debugexporter => /home/jmacd/src/otel/opentelemetry-collector/exporter/debugexporter

replace go.opentelemetry.io/collector/exporter/exporterhelper => /home/jmacd/src/otel/opentelemetry-collector/exporter/exporterhelper

replace go.opentelemetry.io/collector/exporter/exporterhelper/xexporterhelper => /home/jmacd/src/otel/opentelemetry-collector/exporter/exporterhelper/xexporterhelper

replace go.opentelemetry.io/collector/exporter/exportertest => /home/jmacd/src/otel/opentelemetry-collector/exporter/exportertest

replace go.opentelemetry.io/collector/exporter/nopexporter => /home/jmacd/src/otel/opentelemetry-collector/exporter/nopexporter

replace go.opentelemetry.io/collector/exporter/otlpexporter => /home/jmacd/src/otel/opentelemetry-collector/exporter/otlpexporter

replace go.opentelemetry.io/collector/exporter/otlphttpexporter => /home/jmacd/src/otel/opentelemetry-collector/exporter/otlphttpexporter

replace go.opentelemetry.io/collector/exporter/xexporter => /home/jmacd/src/otel/opentelemetry-collector/exporter/xexporter

replace go.opentelemetry.io/collector/extension/extensionauth/extensionauthtest => /home/jmacd/src/otel/opentelemetry-collector/extension/extensionauth/extensionauthtest

replace go.opentelemetry.io/collector/extension/extensioncapabilities => /home/jmacd/src/otel/opentelemetry-collector/extension/extensioncapabilities

replace go.opentelemetry.io/collector/extension/extensionmiddleware => /home/jmacd/src/otel/opentelemetry-collector/extension/extensionmiddleware

replace go.opentelemetry.io/collector/extension/extensionmiddleware/extensionmiddlewaretest => /home/jmacd/src/otel/opentelemetry-collector/extension/extensionmiddleware/extensionmiddlewaretest

replace go.opentelemetry.io/collector/extension/extensiontest => /home/jmacd/src/otel/opentelemetry-collector/extension/extensiontest

replace go.opentelemetry.io/collector/extension/zpagesextension => /home/jmacd/src/otel/opentelemetry-collector/extension/zpagesextension

replace go.opentelemetry.io/collector/extension/memorylimiterextension => /home/jmacd/src/otel/opentelemetry-collector/extension/memorylimiterextension

replace go.opentelemetry.io/collector/extension/xextension => /home/jmacd/src/otel/opentelemetry-collector/extension/xextension

replace go.opentelemetry.io/collector/otelcol => /home/jmacd/src/otel/opentelemetry-collector/otelcol

replace go.opentelemetry.io/collector/otelcol/otelcoltest => /home/jmacd/src/otel/opentelemetry-collector/otelcol/otelcoltest

replace go.opentelemetry.io/collector/pdata/pprofile => /home/jmacd/src/otel/opentelemetry-collector/pdata/pprofile

replace go.opentelemetry.io/collector/pdata/testdata => /home/jmacd/src/otel/opentelemetry-collector/pdata/testdata

replace go.opentelemetry.io/collector/pdata/xpdata => /home/jmacd/src/otel/opentelemetry-collector/pdata/xpdata

replace go.opentelemetry.io/collector/pipeline/xpipeline => /home/jmacd/src/otel/opentelemetry-collector/pipeline/xpipeline

replace go.opentelemetry.io/collector/processor/processortest => /home/jmacd/src/otel/opentelemetry-collector/processor/processortest

replace go.opentelemetry.io/collector/processor/processorhelper => /home/jmacd/src/otel/opentelemetry-collector/processor/processorhelper

replace go.opentelemetry.io/collector/processor/batchprocessor => /home/jmacd/src/otel/opentelemetry-collector/processor/batchprocessor

replace go.opentelemetry.io/collector/processor/memorylimiterprocessor => /home/jmacd/src/otel/opentelemetry-collector/processor/memorylimiterprocessor

replace go.opentelemetry.io/collector/processor/processorhelper/xprocessorhelper => /home/jmacd/src/otel/opentelemetry-collector/processor/processorhelper/xprocessorhelper

replace go.opentelemetry.io/collector/processor/xprocessor => /home/jmacd/src/otel/opentelemetry-collector/processor/xprocessor

replace go.opentelemetry.io/collector/receiver/receiverhelper => /home/jmacd/src/otel/opentelemetry-collector/receiver/receiverhelper

replace go.opentelemetry.io/collector/receiver/nopreceiver => /home/jmacd/src/otel/opentelemetry-collector/receiver/nopreceiver

replace go.opentelemetry.io/collector/receiver/otlpreceiver => /home/jmacd/src/otel/opentelemetry-collector/receiver/otlpreceiver

replace go.opentelemetry.io/collector/receiver/receivertest => /home/jmacd/src/otel/opentelemetry-collector/receiver/receivertest

replace go.opentelemetry.io/collector/receiver/xreceiver => /home/jmacd/src/otel/opentelemetry-collector/receiver/xreceiver

replace go.opentelemetry.io/collector/scraper => /home/jmacd/src/otel/opentelemetry-collector/scraper

replace go.opentelemetry.io/collector/scraper/scraperhelper => /home/jmacd/src/otel/opentelemetry-collector/scraper/scraperhelper

replace go.opentelemetry.io/collector/scraper/scrapertest => /home/jmacd/src/otel/opentelemetry-collector/scraper/scrapertest

replace go.opentelemetry.io/collector/service => /home/jmacd/src/otel/opentelemetry-collector/service

replace go.opentelemetry.io/collector/service/hostcapabilities => /home/jmacd/src/otel/opentelemetry-collector/service/hostcapabilities

replace go.opentelemetry.io/collector/service/telemetry/telemetrytest => /home/jmacd/src/otel/opentelemetry-collector/service/telemetry/telemetrytest

replace go.opentelemetry.io/collector/filter => /home/jmacd/src/otel/opentelemetry-collector/filter

replace go.opentelemetry.io/collector/client => /home/jmacd/src/otel/opentelemetry-collector/client

replace go.opentelemetry.io/collector/featuregate => /home/jmacd/src/otel/opentelemetry-collector/featuregate

replace go.opentelemetry.io/collector/pdata => /home/jmacd/src/otel/opentelemetry-collector/pdata

replace go.opentelemetry.io/collector/component => /home/jmacd/src/otel/opentelemetry-collector/component

replace go.opentelemetry.io/collector/confmap => /home/jmacd/src/otel/opentelemetry-collector/confmap

replace go.opentelemetry.io/collector/confmap/provider/envprovider => /home/jmacd/src/otel/opentelemetry-collector/confmap/provider/envprovider

replace go.opentelemetry.io/collector/confmap/provider/fileprovider => /home/jmacd/src/otel/opentelemetry-collector/confmap/provider/fileprovider

replace go.opentelemetry.io/collector/confmap/provider/httpprovider => /home/jmacd/src/otel/opentelemetry-collector/confmap/provider/httpprovider

replace go.opentelemetry.io/collector/confmap/provider/httpsprovider => /home/jmacd/src/otel/opentelemetry-collector/confmap/provider/httpsprovider

replace go.opentelemetry.io/collector/confmap/provider/yamlprovider => /home/jmacd/src/otel/opentelemetry-collector/confmap/provider/yamlprovider

replace go.opentelemetry.io/collector/config/configauth => /home/jmacd/src/otel/opentelemetry-collector/config/configauth

replace go.opentelemetry.io/collector/config/configopaque => /home/jmacd/src/otel/opentelemetry-collector/config/configopaque

replace go.opentelemetry.io/collector/config/configoptional => /home/jmacd/src/otel/opentelemetry-collector/config/configoptional

replace go.opentelemetry.io/collector/config/configcompression => /home/jmacd/src/otel/opentelemetry-collector/config/configcompression

replace go.opentelemetry.io/collector/config/configretry => /home/jmacd/src/otel/opentelemetry-collector/config/configretry

replace go.opentelemetry.io/collector/config/configtls => /home/jmacd/src/otel/opentelemetry-collector/config/configtls

replace go.opentelemetry.io/collector/config/confignet => /home/jmacd/src/otel/opentelemetry-collector/config/confignet

replace go.opentelemetry.io/collector/config/configmiddleware => /home/jmacd/src/otel/opentelemetry-collector/config/configmiddleware

replace go.opentelemetry.io/collector/consumer => /home/jmacd/src/otel/opentelemetry-collector/consumer

replace go.opentelemetry.io/collector/exporter => /home/jmacd/src/otel/opentelemetry-collector/exporter

replace go.opentelemetry.io/collector/extension => /home/jmacd/src/otel/opentelemetry-collector/extension

replace go.opentelemetry.io/collector/extension/extensionauth => /home/jmacd/src/otel/opentelemetry-collector/extension/extensionauth

replace go.opentelemetry.io/collector/pipeline => /home/jmacd/src/otel/opentelemetry-collector/pipeline

replace go.opentelemetry.io/collector/processor => /home/jmacd/src/otel/opentelemetry-collector/processor

replace go.opentelemetry.io/collector/receiver => /home/jmacd/src/otel/opentelemetry-collector/receiver
