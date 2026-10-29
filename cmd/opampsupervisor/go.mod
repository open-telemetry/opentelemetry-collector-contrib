module github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor

go 1.26.0

require (
	github.com/google/uuid v1.6.0
	github.com/open-telemetry/opamp-go v0.25.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension v0.164.1
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/bearertokenauthextension v0.164.1
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension v0.164.1
	github.com/sigstore/sigstore-go v1.3.0
	github.com/stretchr/testify v1.12.1
	go.opentelemetry.io/collector/component v1.70.0
	go.opentelemetry.io/collector/component/componenttest v0.164.0
	go.opentelemetry.io/collector/config/confighttp v0.164.0
	go.opentelemetry.io/collector/config/confignet v1.70.0
	go.opentelemetry.io/collector/config/configopaque v1.70.0
	go.opentelemetry.io/collector/config/configoptional v1.70.0
	go.opentelemetry.io/collector/config/configtelemetry v0.164.0
	go.opentelemetry.io/collector/config/configtls v1.70.0
	go.opentelemetry.io/collector/confmap v1.70.0
	go.opentelemetry.io/collector/confmap/provider/envprovider v1.70.0
	go.opentelemetry.io/collector/confmap/provider/fileprovider v1.70.0
	go.opentelemetry.io/collector/consumer/consumertest v0.164.0
	go.opentelemetry.io/collector/extension v1.70.0
	go.opentelemetry.io/collector/extension/extensionauth v1.70.0
	go.opentelemetry.io/collector/extension/extensiontest v0.164.0
	go.opentelemetry.io/collector/featuregate v1.70.0
	go.opentelemetry.io/collector/pdata v1.70.0
	go.opentelemetry.io/collector/receiver/otlpreceiver v0.164.0
	go.opentelemetry.io/collector/receiver/receivertest v0.164.0
	go.opentelemetry.io/collector/service v0.164.0
	go.opentelemetry.io/contrib/bridges/otelzap v0.21.0
	go.opentelemetry.io/contrib/otelconf v0.27.0
	go.opentelemetry.io/otel v1.47.0
	go.opentelemetry.io/otel/log v1.47.0
	go.opentelemetry.io/otel/metric v1.47.0
	go.opentelemetry.io/otel/sdk/metric v1.47.0
	go.opentelemetry.io/otel/trace v1.47.0
	go.uber.org/goleak v1.3.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.28.0
	golang.org/x/sys v0.49.0
	google.golang.org/protobuf v1.36.12
	gopkg.in/yaml.v3 v3.0.1
)

require (
	cloud.google.com/go/compute/metadata v0.10.0 // indirect
	github.com/GehirnInc/crypt v0.0.0-20230320061759-8cc1b52080c5 // indirect
	github.com/Microsoft/go-winio v0.6.4 // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/aws/aws-sdk-go-v2 v1.47.4 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.33.10 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.20.10 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.20.4 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.7 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.7 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.5.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.22 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.14.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.10.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.38.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.43.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.51.5 // indirect
	github.com/aws/smithy-go v1.28.6 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver v3.5.1+incompatible // indirect
	github.com/brunoscheufler/aws-ecs-metadata-go v0.0.0-20221221133751-67e37ae746cd // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cyberphone/json-canonicalization v0.0.0-20241213102144-19d51d7fe467 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/digitorus/pkcs7 v0.0.0-20230818184609-3a137a874352 // indirect
	github.com/digitorus/timestamp v0.0.0-20231217203849-220c5c2851b7 // indirect
	github.com/elastic/proxy-connect-dialer-go v0.1.1 // indirect
	github.com/felixge/httpsnoop v1.1.0 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20251226215517-609e4778396f // indirect
	github.com/fsnotify/fsnotify v1.10.1 // indirect
	github.com/fxamacker/cbor/v2 v2.9.6 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/analysis v1.0.0 // indirect
	github.com/go-openapi/errors v0.22.9 // indirect
	github.com/go-openapi/jsonpointer v1.0.1 // indirect
	github.com/go-openapi/jsonreference v1.0.1 // indirect
	github.com/go-openapi/loads v0.25.2 // indirect
	github.com/go-openapi/runtime v0.33.2 // indirect
	github.com/go-openapi/runtime/server-middleware v0.33.2 // indirect
	github.com/go-openapi/spec v1.0.0 // indirect
	github.com/go-openapi/strfmt v0.27.2 // indirect
	github.com/go-openapi/swag v0.29.2 // indirect
	github.com/go-openapi/swag/cmdutils v0.29.2 // indirect
	github.com/go-openapi/swag/conv v0.29.2 // indirect
	github.com/go-openapi/swag/fileutils v0.29.2 // indirect
	github.com/go-openapi/swag/jsonutils v0.29.2 // indirect
	github.com/go-openapi/swag/loading v0.29.2 // indirect
	github.com/go-openapi/swag/mangling v0.29.2 // indirect
	github.com/go-openapi/swag/netutils v0.29.2 // indirect
	github.com/go-openapi/swag/pools v0.29.2 // indirect
	github.com/go-openapi/swag/stringutils v0.29.2 // indirect
	github.com/go-openapi/swag/typeutils v0.29.2 // indirect
	github.com/go-openapi/swag/yamlutils v0.29.2 // indirect
	github.com/go-openapi/validate v1.0.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/certificate-transparency-go v1.3.3 // indirect
	github.com/google/go-containerregistry v0.21.7 // indirect
	github.com/google/go-tpm v0.9.8 // indirect
	github.com/gorilla/websocket v1.5.4-0.20250319132907-e064f32e3674 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.31.0 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/in-toto/attestation v1.2.0 // indirect
	github.com/in-toto/in-toto-golang v0.11.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.20.3 // indirect
	github.com/knadh/koanf/maps v0.1.3 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.1 // indirect
	github.com/knadh/koanf/v2 v2.3.8 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/oklog/ulid/v2 v2.1.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/internal/basicauth v0.164.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/internal/credentialsfile v0.164.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/internal/secretprovider v0.164.1 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.33 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.25.0 // indirect
	github.com/prometheus/client_model v0.6.3 // indirect
	github.com/prometheus/common v0.72.1 // indirect
	github.com/prometheus/otlptranslator v1.0.0 // indirect
	github.com/prometheus/procfs v0.22.0 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/secure-systems-lab/go-securesystemslib v0.11.0 // indirect
	github.com/shibumi/go-pathspec v1.3.0 // indirect
	github.com/sigstore/protobuf-specs v0.5.1 // indirect
	github.com/sigstore/rekor v1.5.3 // indirect
	github.com/sigstore/rekor-tiles/v2 v2.3.0 // indirect
	github.com/sigstore/sigstore v1.10.8 // indirect
	github.com/sigstore/timestamp-authority/v2 v2.1.3 // indirect
	github.com/tg123/go-htpasswd v1.2.5 // indirect
	github.com/theupdateframework/go-tuf/v2 v2.4.2 // indirect
	github.com/transparency-dev/formats v0.1.1 // indirect
	github.com/transparency-dev/merkle v0.0.2 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/client v1.70.0 // indirect
	go.opentelemetry.io/collector/component/componentstatus v0.164.0 // indirect
	go.opentelemetry.io/collector/config/configauth v1.70.0 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.70.0 // indirect
	go.opentelemetry.io/collector/config/configgrpc v1.70.0 // indirect
	go.opentelemetry.io/collector/config/configmiddleware v1.70.0 // indirect
	go.opentelemetry.io/collector/consumer v1.70.0 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.164.0 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.164.0 // indirect
	go.opentelemetry.io/collector/extension/extensioncapabilities v0.164.0 // indirect
	go.opentelemetry.io/collector/extension/extensionmiddleware v0.164.0 // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.164.0 // indirect
	go.opentelemetry.io/collector/internal/sharedcomponent v0.164.0 // indirect
	go.opentelemetry.io/collector/internal/statusutil v0.164.0 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.164.0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.164.0 // indirect
	go.opentelemetry.io/collector/pipeline v1.70.0 // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.164.0 // indirect
	go.opentelemetry.io/collector/receiver v1.70.0 // indirect
	go.opentelemetry.io/collector/receiver/receiverhelper v0.164.0 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.164.0 // indirect
	go.opentelemetry.io/contrib/detectors/aws/ec2/v2 v2.6.0 // indirect
	go.opentelemetry.io/contrib/detectors/aws/ecs v1.47.0 // indirect
	go.opentelemetry.io/contrib/detectors/aws/eks v1.47.0 // indirect
	go.opentelemetry.io/contrib/detectors/azure/azurevm v0.19.0 // indirect
	go.opentelemetry.io/contrib/detectors/gcp v1.47.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.71.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.72.0 // indirect
	go.opentelemetry.io/contrib/propagators/autoprop v0.72.0 // indirect
	go.opentelemetry.io/contrib/propagators/aws v1.47.0 // indirect
	go.opentelemetry.io/contrib/propagators/b3 v1.47.0 // indirect
	go.opentelemetry.io/contrib/propagators/jaeger v1.47.0 // indirect
	go.opentelemetry.io/contrib/propagators/ot v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc v0.23.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.23.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.69.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutlog v0.23.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.47.0 // indirect
	go.opentelemetry.io/otel/sdk v1.47.0 // indirect
	go.opentelemetry.io/otel/sdk/log v1.47.0 // indirect
	go.opentelemetry.io/proto/otlp v1.11.1 // indirect
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/crypto v0.58.0 // indirect
	golang.org/x/exp v0.0.0-20261011100332-faecfb24eb49 // indirect
	golang.org/x/mod v0.42.0 // indirect
	golang.org/x/net v0.61.0 // indirect
	golang.org/x/oauth2 v0.37.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/term v0.47.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	golang.org/x/time v0.16.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260928230214-8a89bd6388cc // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260928230214-8a89bd6388cc // indirect
	google.golang.org/grpc v1.84.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	k8s.io/apimachinery v0.37.1 // indirect
	k8s.io/client-go v0.37.1 // indirect
	k8s.io/klog/v2 v2.140.0 // indirect
	k8s.io/kube-openapi v0.0.0-20260821135717-be32def86098 // indirect
	k8s.io/utils v0.0.0-20260707023825-cf1189d6abe3 // indirect
	sigs.k8s.io/json v0.0.0-20260909141634-11ed52e25bc5 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.4.2 // indirect
	sigs.k8s.io/yaml v1.6.0 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/bearertokenauthextension => ../../extension/bearertokenauthextension

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension => ../../extension/basicauthextension

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/internal/basicauth => ../../extension/internal/basicauth

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension => ../../extension/oauth2clientauthextension

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/internal/credentialsfile => ../../extension/internal/credentialsfile

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/internal/secretprovider => ../../extension/internal/secretprovider
