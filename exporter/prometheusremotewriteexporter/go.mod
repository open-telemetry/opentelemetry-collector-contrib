module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter

go 1.23.0

require (
	github.com/cenkalti/backoff/v5 v5.0.2
	github.com/gogo/protobuf v1.3.2
	github.com/golang/snappy v1.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.130.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.130.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry v0.130.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite v0.130.0
	github.com/prometheus/otlptranslator v0.0.0-20250620074007-94f535e0c588
	github.com/prometheus/prometheus v0.304.3-0.20250703114031-419d436a447a
	github.com/stretchr/testify v1.10.0
	github.com/tidwall/wal v1.1.8
	go.opentelemetry.io/collector/component v1.36.1-0.20250715222903-0a7598ec1e19
	go.opentelemetry.io/collector/component/componenttest v0.130.1-0.20250715222903-0a7598ec1e19
	go.opentelemetry.io/collector/config/confighttp v0.130.1-0.20250715222903-0a7598ec1e19
	go.opentelemetry.io/collector/config/configopaque v1.36.1-0.20250715222903-0a7598ec1e19
	go.opentelemetry.io/collector/config/configretry v1.36.1-0.20250715222903-0a7598ec1e19
	go.opentelemetry.io/collector/config/configtls v1.36.1-0.20250715222903-0a7598ec1e19
	go.opentelemetry.io/collector/confmap v1.36.1-0.20250715222903-0a7598ec1e19
	go.opentelemetry.io/collector/confmap/xconfmap v0.130.1-0.20250715222903-0a7598ec1e19
	go.opentelemetry.io/collector/consumer/consumererror v0.130.1-0.20250715222903-0a7598ec1e19
	go.opentelemetry.io/collector/exporter v0.130.1-0.20250715222903-0a7598ec1e19
	go.opentelemetry.io/collector/exporter/exportertest v0.130.1-0.20250715222903-0a7598ec1e19
	go.opentelemetry.io/collector/featuregate v1.36.1-0.20250715222903-0a7598ec1e19
	go.opentelemetry.io/collector/pdata v1.36.1-0.20250715222903-0a7598ec1e19
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.62.0
	go.opentelemetry.io/otel v1.37.0
	go.opentelemetry.io/otel/metric v1.37.0
	go.opentelemetry.io/otel/sdk v1.37.0
	go.opentelemetry.io/otel/sdk/metric v1.37.0
	go.opentelemetry.io/otel/trace v1.37.0
	go.uber.org/goleak v1.3.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.0
)

require (
	cloud.google.com/go/auth v0.16.2 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.7.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.18.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.10.1 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.11.1 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.4.2 // indirect
	github.com/alecthomas/units v0.0.0-20240927000941-0f3dac36c52b // indirect
	github.com/aws/aws-sdk-go-v2 v1.36.3 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.29.14 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.17.67 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.30 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.34 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.34 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.12.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.12.15 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.25.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.30.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.33.19 // indirect
	github.com/aws/smithy-go v1.22.2 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20250323135004-b31fac66206e // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/golang-jwt/jwt/v5 v5.2.2 // indirect
	github.com/google/go-tpm v0.9.5 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.6 // indirect
	github.com/googleapis/gax-go/v2 v2.14.2 // indirect
	github.com/grafana/regexp v0.0.0-20240518133315-a468a5bfb3bc // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.2.2 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus v0.130.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_golang v1.22.0 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.65.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/prometheus/sigv4 v0.2.0 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/tidwall/gjson v1.10.2 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	github.com/tidwall/tinylru v1.1.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/client v1.36.1-0.20250715222903-0a7598ec1e19 // indirect
	go.opentelemetry.io/collector/config/configauth v0.130.1-0.20250715222903-0a7598ec1e19 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.36.1-0.20250715222903-0a7598ec1e19 // indirect
	go.opentelemetry.io/collector/config/configmiddleware v0.130.1-0.20250715222903-0a7598ec1e19 // indirect
	go.opentelemetry.io/collector/config/configoptional v0.130.1-0.20250715222903-0a7598ec1e19 // indirect
	go.opentelemetry.io/collector/consumer v1.36.1-0.20250715222903-0a7598ec1e19 // indirect
	go.opentelemetry.io/collector/consumer/consumertest v0.130.1-0.20250715222903-0a7598ec1e19 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.130.1-0.20250715222903-0a7598ec1e19 // indirect
	go.opentelemetry.io/collector/exporter/xexporter v0.130.1-0.20250715222903-0a7598ec1e19 // indirect
	go.opentelemetry.io/collector/extension v1.36.1-0.20250715222903-0a7598ec1e19 // indirect
	go.opentelemetry.io/collector/extension/extensionauth v1.36.1-0.20250715222903-0a7598ec1e19 // indirect
	go.opentelemetry.io/collector/extension/extensionmiddleware v0.130.1-0.20250715222903-0a7598ec1e19 // indirect
	go.opentelemetry.io/collector/extension/xextension v0.130.1-0.20250715222903-0a7598ec1e19 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.130.1-0.20250715222903-0a7598ec1e19 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.130.1-0.20250715222903-0a7598ec1e19 // indirect
	go.opentelemetry.io/collector/pdata/xpdata v0.130.1-0.20250715222903-0a7598ec1e19 // indirect
	go.opentelemetry.io/collector/pipeline v0.130.1-0.20250715222903-0a7598ec1e19 // indirect
	go.opentelemetry.io/collector/receiver v1.36.1-0.20250715222903-0a7598ec1e19 // indirect
	go.opentelemetry.io/collector/receiver/receivertest v0.130.1-0.20250715222903-0a7598ec1e19 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.130.1-0.20250715222903-0a7598ec1e19 // indirect
	go.opentelemetry.io/collector/semconv v0.128.0 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.12.0 // indirect
	go.opentelemetry.io/otel/log v0.13.0 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.40.0 // indirect
	golang.org/x/net v0.42.0 // indirect
	golang.org/x/oauth2 v0.30.0 // indirect
	golang.org/x/sys v0.34.0 // indirect
	golang.org/x/text v0.27.0 // indirect
	golang.org/x/time v0.12.0 // indirect
	google.golang.org/api v0.238.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250603155806-513f23925822 // indirect
	google.golang.org/grpc v1.74.2 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apimachinery v0.32.3 // indirect
	k8s.io/client-go v0.32.3 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/utils v0.0.0-20241104100929-3ea5e8cea738 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry => ../../pkg/resourcetotelemetry

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite => ../../pkg/translator/prometheusremotewrite

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus => ../../pkg/translator/prometheus
