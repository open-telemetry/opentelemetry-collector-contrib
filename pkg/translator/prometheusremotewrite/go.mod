module github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite

go 1.24.0

require (
	github.com/cespare/xxhash/v2 v2.3.0
	github.com/google/go-cmp v0.7.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.140.1
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus v0.140.1
	github.com/prometheus/common v0.67.3
	github.com/prometheus/otlptranslator v1.0.0
	github.com/prometheus/prometheus v0.307.3
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/pdata v1.46.1-0.20251120204106-2e9c82787618
	go.opentelemetry.io/otel v1.38.0
	go.uber.org/goleak v1.3.0
	go.uber.org/multierr v1.11.0
)

require (
	cloud.google.com/go/auth v0.16.5 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.19.1 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.12.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.11.2 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.5.0 // indirect
	github.com/alecthomas/units v0.0.0-20240927000941-0f3dac36c52b // indirect
	github.com/aws/aws-sdk-go-v2 v1.39.2 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.31.12 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.18.16 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.9 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.9 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.9 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.29.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.35.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.38.6 // indirect
	github.com/aws/smithy-go v1.23.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dennwc/varint v1.0.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.0 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.6 // indirect
	github.com/googleapis/gax-go/v2 v2.15.0 // indirect
	github.com/grafana/regexp v0.0.0-20250905093917-f7b3be9d1853 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_golang v1.23.2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/procfs v0.16.1 // indirect
	github.com/prometheus/sigv4 v0.2.1 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/featuregate v1.46.1-0.20251120204106-2e9c82787618 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.140.1-0.20251120204106-2e9c82787618 // indirect
	go.opentelemetry.io/collector/semconv v0.128.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.63.0 // indirect
	go.opentelemetry.io/otel/metric v1.38.0 // indirect
	go.opentelemetry.io/otel/trace v1.38.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
	golang.org/x/crypto v0.45.0 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/oauth2 v0.32.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	golang.org/x/time v0.13.0 // indirect
	google.golang.org/api v0.250.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251022142026-3a174f9686a8 // indirect
	google.golang.org/grpc v1.77.0 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apimachinery v0.34.1 // indirect
	k8s.io/client-go v0.34.1 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/utils v0.0.0-20250604170112-4c0f3b243397 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../../internal/common

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../golden

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus => ../prometheus

replace go.opentelemetry.io/collector => /home/jmacd/src/otel/opentelemetry-collector

replace go.opentelemetry.io/collector/internal/memorylimiter => /home/jmacd/src/otel/opentelemetry-collector/internal/memorylimiter

replace go.opentelemetry.io/collector/internal/fanoutconsumer => /home/jmacd/src/otel/opentelemetry-collector/internal/fanoutconsumer

replace go.opentelemetry.io/collector/internal/sharedcomponent => /home/jmacd/src/otel/opentelemetry-collector/internal/sharedcomponent

replace go.opentelemetry.io/collector/internal/telemetry => /home/jmacd/src/otel/opentelemetry-collector/internal/telemetry

replace go.opentelemetry.io/collector/internal/testutil => /home/jmacd/src/otel/opentelemetry-collector/internal/testutil

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
