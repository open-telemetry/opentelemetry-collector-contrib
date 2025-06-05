module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver

go 1.23.0

require (
	github.com/cespare/xxhash/v2 v2.3.0
	github.com/gogo/protobuf v1.3.2
	github.com/golang/snappy v1.0.0
	github.com/hashicorp/golang-lru/v2 v2.0.7
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics v0.127.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.127.0
	github.com/prometheus/prometheus v0.304.1
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v1.33.1-0.20250605124916-efac6c7d44f1
	go.opentelemetry.io/collector/component/componentstatus v0.127.1-0.20250605124916-efac6c7d44f1
	go.opentelemetry.io/collector/component/componenttest v0.127.1-0.20250605124916-efac6c7d44f1
	go.opentelemetry.io/collector/config/confighttp v0.127.1-0.20250605124916-efac6c7d44f1
	go.opentelemetry.io/collector/confmap v1.33.1-0.20250605124916-efac6c7d44f1
	go.opentelemetry.io/collector/consumer v1.33.1-0.20250605124916-efac6c7d44f1
	go.opentelemetry.io/collector/consumer/consumertest v0.127.1-0.20250605124916-efac6c7d44f1
	go.opentelemetry.io/collector/pdata v1.33.1-0.20250605124916-efac6c7d44f1
	go.opentelemetry.io/collector/receiver v1.33.1-0.20250605124916-efac6c7d44f1
	go.opentelemetry.io/collector/receiver/receiverhelper v0.127.1-0.20250605124916-efac6c7d44f1
	go.opentelemetry.io/collector/receiver/receivertest v0.127.1-0.20250605124916-efac6c7d44f1
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.0
)

require (
	cloud.google.com/go/auth v0.16.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute/metadata v0.6.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.18.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.9.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.11.1 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.4.2 // indirect
	github.com/alecthomas/units v0.0.0-20240927000941-0f3dac36c52b // indirect
	github.com/aws/aws-sdk-go v1.55.7 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dennwc/varint v1.0.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20250323135004-b31fac66206e // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/golang-jwt/jwt/v5 v5.2.2 // indirect
	github.com/google/go-tpm v0.9.5 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.6 // indirect
	github.com/googleapis/gax-go/v2 v2.14.1 // indirect
	github.com/grafana/regexp v0.0.0-20240518133315-a468a5bfb3bc // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.2.0 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.127.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor v0.127.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_golang v1.22.0 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.63.0 // indirect
	github.com/prometheus/otlptranslator v0.0.0-20250320144820-d800c8b0eb07 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/prometheus/sigv4 v0.1.2 // indirect
	github.com/puzpuzpuz/xsync/v3 v3.5.1 // indirect
	github.com/rs/cors v1.11.1 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/client v1.33.1-0.20250605124916-efac6c7d44f1 // indirect
	go.opentelemetry.io/collector/config/configauth v0.127.1-0.20250605124916-efac6c7d44f1 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.33.1-0.20250605124916-efac6c7d44f1 // indirect
	go.opentelemetry.io/collector/config/configmiddleware v0.127.1-0.20250605124916-efac6c7d44f1 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.33.1-0.20250605124916-efac6c7d44f1 // indirect
	go.opentelemetry.io/collector/config/configtls v1.33.1-0.20250605124916-efac6c7d44f1 // indirect
	go.opentelemetry.io/collector/confmap/xconfmap v0.127.1-0.20250605124916-efac6c7d44f1 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.127.1-0.20250605124916-efac6c7d44f1 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.127.1-0.20250605124916-efac6c7d44f1 // indirect
	go.opentelemetry.io/collector/extension/extensionauth v1.33.1-0.20250605124916-efac6c7d44f1 // indirect
	go.opentelemetry.io/collector/extension/extensionmiddleware v0.127.1-0.20250605124916-efac6c7d44f1 // indirect
	go.opentelemetry.io/collector/featuregate v1.33.1-0.20250605124916-efac6c7d44f1 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.127.1-0.20250605124916-efac6c7d44f1 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.127.1-0.20250605124916-efac6c7d44f1 // indirect
	go.opentelemetry.io/collector/pipeline v0.127.1-0.20250605124916-efac6c7d44f1 // indirect
	go.opentelemetry.io/collector/processor v1.33.1-0.20250605124916-efac6c7d44f1 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.127.1-0.20250605124916-efac6c7d44f1 // indirect
	go.opentelemetry.io/collector/semconv v0.127.1-0.20250605124916-efac6c7d44f1 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.11.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace v0.60.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.61.0 // indirect
	go.opentelemetry.io/otel v1.36.0 // indirect
	go.opentelemetry.io/otel/log v0.12.2 // indirect
	go.opentelemetry.io/otel/metric v1.36.0 // indirect
	go.opentelemetry.io/otel/sdk v1.36.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.36.0 // indirect
	go.opentelemetry.io/otel/trace v1.36.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.38.0 // indirect
	golang.org/x/net v0.40.0 // indirect
	golang.org/x/oauth2 v0.29.0 // indirect
	golang.org/x/sys v0.33.0 // indirect
	golang.org/x/text v0.25.0 // indirect
	golang.org/x/time v0.11.0 // indirect
	google.golang.org/api v0.230.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250414145226-207652e42e2e // indirect
	google.golang.org/grpc v1.72.2 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apimachinery v0.32.3 // indirect
	k8s.io/client-go v0.32.3 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/utils v0.0.0-20241104100929-3ea5e8cea738 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics => ../../internal/exp/metrics

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor => ../../processor/deltatocumulativeprocessor
