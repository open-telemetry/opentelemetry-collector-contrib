module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver

go 1.24.0

require (
	github.com/google/go-cmp v0.7.0
	github.com/google/uuid v1.6.0
	github.com/iancoleman/strcase v0.3.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/k8sleaderelector v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.139.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest v0.139.0
	github.com/openshift/api v0.0.0-20251015095338-264e80a2b6e7
	github.com/openshift/client-go v0.0.0-20251015124057-db0dee36e235
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/component v1.46.0
	go.opentelemetry.io/collector/component/componentstatus v0.140.0
	go.opentelemetry.io/collector/component/componenttest v0.140.0
	go.opentelemetry.io/collector/config/configoptional v1.46.0
	go.opentelemetry.io/collector/confmap v1.46.0
	go.opentelemetry.io/collector/confmap/xconfmap v0.140.0
	go.opentelemetry.io/collector/consumer v1.46.0
	go.opentelemetry.io/collector/consumer/consumertest v0.140.0
	go.opentelemetry.io/collector/filter v0.139.1-0.20251117090521-8825f4331bfc
	go.opentelemetry.io/collector/pdata v1.46.0
	go.opentelemetry.io/collector/pipeline v1.46.0
	go.opentelemetry.io/collector/receiver v1.46.0
	go.opentelemetry.io/collector/receiver/otlpreceiver v0.139.1-0.20251117090521-8825f4331bfc
	go.opentelemetry.io/collector/receiver/receiverhelper v0.140.0
	go.opentelemetry.io/collector/receiver/receivertest v0.140.0
	go.opentelemetry.io/otel v1.38.0
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.0
	k8s.io/api v0.34.1
	k8s.io/apimachinery v0.34.1
	k8s.io/client-go v0.34.1
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/docker v28.5.2+incompatible // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/emicklei/go-restful/v3 v3.12.2 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20250903184740-5d135037bd4d // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/gnostic-models v0.7.0 // indirect
	github.com/google/go-tpm v0.9.7 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.18.1 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/sys/sequential v0.6.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/mostynb/go-grpc-compression v1.2.3 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.139.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0-rc4 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector v0.140.0 // indirect
	go.opentelemetry.io/collector/client v1.46.0 // indirect
	go.opentelemetry.io/collector/config/configauth v1.46.0 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.46.0 // indirect
	go.opentelemetry.io/collector/config/configgrpc v0.140.0 // indirect
	go.opentelemetry.io/collector/config/confighttp v0.140.0 // indirect
	go.opentelemetry.io/collector/config/configmiddleware v1.46.0 // indirect
	go.opentelemetry.io/collector/config/confignet v1.46.0 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.46.0 // indirect
	go.opentelemetry.io/collector/config/configtls v1.46.0 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.140.0 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.140.0 // indirect
	go.opentelemetry.io/collector/extension v1.46.0 // indirect
	go.opentelemetry.io/collector/extension/extensionauth v1.46.0 // indirect
	go.opentelemetry.io/collector/extension/extensionmiddleware v0.140.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.46.0 // indirect
	go.opentelemetry.io/collector/internal/sharedcomponent v0.140.0 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.140.0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.140.0 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.140.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.63.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.63.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.30.0 // indirect
	go.opentelemetry.io/otel/metric v1.38.0 // indirect
	go.opentelemetry.io/otel/sdk v1.38.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.38.0 // indirect
	go.opentelemetry.io/otel/trace v1.38.0 // indirect
	go.opentelemetry.io/proto/otlp v1.3.1 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.45.0 // indirect
	golang.org/x/mod v0.29.0 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/oauth2 v0.32.0 // indirect
	golang.org/x/sync v0.18.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/term v0.37.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	golang.org/x/time v0.9.0 // indirect
	golang.org/x/tools v0.38.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251022142026-3a174f9686a8 // indirect
	google.golang.org/grpc v1.77.0 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.12.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/kube-openapi v0.0.0-20250710124328-f3f2b991d03b // indirect
	k8s.io/utils v0.0.0-20250604170112-4c0f3b243397 // indirect
	sigs.k8s.io/json v0.0.0-20241014173422-cfa47c3a1cc8 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.3.0 // indirect
	sigs.k8s.io/yaml v1.6.0 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig => ../../internal/k8sconfig

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata => ../../pkg/experimentalmetricmetadata

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest => ../../pkg/xk8stest

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent => ../../internal/sharedcomponent

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/k8sleaderelector => ../../extension/k8sleaderelector

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
