module github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor

go 1.23.7

require (
	github.com/cenkalti/backoff/v4 v4.3.0
	github.com/google/uuid v1.6.0
	github.com/knadh/koanf/maps v0.1.2
	github.com/knadh/koanf/parsers/yaml v1.0.0
	github.com/knadh/koanf/providers/file v1.2.0
	github.com/knadh/koanf/providers/rawbytes v1.0.0
	github.com/knadh/koanf/v2 v2.2.0
	github.com/open-telemetry/opamp-go v0.19.0
	github.com/open-telemetry/opentelemetry-collector-contrib/testbed v0.125.0
	github.com/sigstore/cosign/v2 v2.4.0
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v1.31.1-0.20250430101735-2ecd0b7f95cd
	go.opentelemetry.io/collector/config/configopaque v1.31.1-0.20250430101735-2ecd0b7f95cd
	go.opentelemetry.io/collector/config/configtelemetry v0.125.1-0.20250430101735-2ecd0b7f95cd
	go.opentelemetry.io/collector/config/configtls v1.31.1-0.20250430101735-2ecd0b7f95cd
	go.opentelemetry.io/collector/confmap v1.31.1-0.20250430101735-2ecd0b7f95cd
	go.opentelemetry.io/collector/confmap/provider/envprovider v1.31.1-0.20250430101735-2ecd0b7f95cd
	go.opentelemetry.io/collector/confmap/provider/fileprovider v1.31.1-0.20250430101735-2ecd0b7f95cd
	go.opentelemetry.io/collector/pdata v1.31.1-0.20250430101735-2ecd0b7f95cd
	go.opentelemetry.io/collector/semconv v0.125.1-0.20250430101735-2ecd0b7f95cd
	go.opentelemetry.io/collector/service v0.125.1-0.20250430101735-2ecd0b7f95cd
	go.opentelemetry.io/contrib/bridges/otelzap v0.10.0
	go.opentelemetry.io/contrib/otelconf v0.15.0
	go.opentelemetry.io/otel v1.35.0
	go.opentelemetry.io/otel/log v0.11.0
	go.opentelemetry.io/otel/trace v1.35.0
	go.uber.org/goleak v1.3.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.0
	golang.org/x/sys v0.32.0
	google.golang.org/protobuf v1.36.6
	gopkg.in/yaml.v3 v3.0.1
)

require (
	cloud.google.com/go/compute/metadata v0.6.0 // indirect
	filippo.io/edwards25519 v1.1.0 // indirect
	github.com/AliyunContainerService/ack-ram-tool/pkg/credentials/alibabacloudsdkgo/helper v0.2.0 // indirect
	github.com/Azure/azure-sdk-for-go v68.0.0+incompatible // indirect
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest v0.11.29 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.23 // indirect
	github.com/Azure/go-autorest/autorest/azure/auth v0.5.12 // indirect
	github.com/Azure/go-autorest/autorest/azure/cli v0.4.6 // indirect
	github.com/Azure/go-autorest/autorest/date v0.3.0 // indirect
	github.com/Azure/go-autorest/logger v0.2.1 // indirect
	github.com/Azure/go-autorest/tracing v0.6.0 // indirect
	github.com/ProtonMail/go-crypto v0.0.0-20230923063757-afb1ddc0824c // indirect
	github.com/ThalesIgnite/crypto11 v1.2.5 // indirect
	github.com/alecthomas/participle/v2 v2.1.4 // indirect
	github.com/alibabacloud-go/alibabacloud-gateway-spi v0.0.4 // indirect
	github.com/alibabacloud-go/cr-20160607 v1.0.1 // indirect
	github.com/alibabacloud-go/cr-20181201 v1.0.10 // indirect
	github.com/alibabacloud-go/darabonba-openapi v0.2.1 // indirect
	github.com/alibabacloud-go/debug v1.0.0 // indirect
	github.com/alibabacloud-go/endpoint-util v1.1.1 // indirect
	github.com/alibabacloud-go/openapi-util v0.1.0 // indirect
	github.com/alibabacloud-go/tea v1.2.1 // indirect
	github.com/alibabacloud-go/tea-utils v1.4.5 // indirect
	github.com/alibabacloud-go/tea-xml v1.1.3 // indirect
	github.com/aliyun/credentials-go v1.3.1 // indirect
	github.com/antchfx/xmlquery v1.4.4 // indirect
	github.com/antchfx/xpath v1.3.4 // indirect
	github.com/apache/thrift v0.21.0 // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/aws/aws-sdk-go-v2 v1.30.3 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.27.27 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.17.27 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.16.11 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.3.15 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.6.15 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecr v1.20.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecrpublic v1.18.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.11.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.11.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.22.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.26.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.30.3 // indirect
	github.com/aws/smithy-go v1.20.3 // indirect
	github.com/awslabs/amazon-ecr-credential-helper/ecr-login v0.0.0-20231024185945-8841054dbdb8 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver v3.5.1+incompatible // indirect
	github.com/cenkalti/backoff/v5 v5.0.2 // indirect
	github.com/census-instrumentation/opencensus-proto v0.4.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/chrismellard/docker-credential-acr-env v0.0.0-20230304212654-82a0ddb27589 // indirect
	github.com/clbanning/mxj/v2 v2.7.0 // indirect
	github.com/cloudflare/circl v1.3.7 // indirect
	github.com/common-nighthawk/go-figure v0.0.0-20210622060536-734e95fb86be // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.14.3 // indirect
	github.com/coreos/go-oidc/v3 v3.11.0 // indirect
	github.com/cyberphone/json-canonicalization v0.0.0-20231011164504-785e29786b46 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/digitorus/pkcs7 v0.0.0-20230818184609-3a137a874352 // indirect
	github.com/digitorus/timestamp v0.0.0-20231217203849-220c5c2851b7 // indirect
	github.com/dimchansky/utfbom v1.1.1 // indirect
	github.com/docker/cli v24.0.7+incompatible // indirect
	github.com/docker/distribution v2.8.3+incompatible // indirect
	github.com/docker/docker v28.0.1+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.8.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/ebitengine/purego v0.8.2 // indirect
	github.com/elastic/go-grok v0.3.1 // indirect
	github.com/elastic/lunes v0.1.0 // indirect
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect
	github.com/expr-lang/expr v1.17.2 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/go-chi/chi v4.1.2+incompatible // indirect
	github.com/go-jose/go-jose/v3 v3.0.3 // indirect
	github.com/go-jose/go-jose/v4 v4.0.4 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-openapi/analysis v0.23.0 // indirect
	github.com/go-openapi/errors v0.22.0 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.21.0 // indirect
	github.com/go-openapi/loads v0.22.0 // indirect
	github.com/go-openapi/runtime v0.28.0 // indirect
	github.com/go-openapi/spec v0.21.0 // indirect
	github.com/go-openapi/strfmt v0.23.0 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-openapi/validate v0.24.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/gogo/googleapis v1.4.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.5.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/certificate-transparency-go v1.2.1 // indirect
	github.com/google/gnostic-models v0.6.9-0.20230804172637-c7be7c783f49 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/go-containerregistry v0.20.1 // indirect
	github.com/google/go-github/v55 v55.0.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.26.3 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.7 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/hashicorp/hcl v1.0.1-vault-5 // indirect
	github.com/iancoleman/strcase v0.3.0 // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/in-toto/in-toto-golang v0.9.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jaegertracing/jaeger-idl v0.5.0 // indirect
	github.com/jedisct1/go-minisign v0.0.0-20230811132847-661be99b8267 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/jonboulle/clockwork v0.5.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/leodido/go-syslog/v4 v4.2.0 // indirect
	github.com/leodido/ragel-machinery v0.0.0-20190525184631-5f46317e436b // indirect
	github.com/letsencrypt/boulder v0.0.0-20240620165639-de9c06129bec // indirect
	github.com/lightstep/go-expohisto v1.0.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20240226150601-1dcf7310316a // indirect
	github.com/magefile/mage v1.15.0 // indirect
	github.com/magiconair/properties v1.8.10 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/miekg/pkcs11 v1.1.1 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.5.1-0.20231216201459-8508981c8b6c // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mostynb/go-grpc-compression v1.2.3 // indirect
	github.com/mozillazg/docker-credential-acr-helper v0.3.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/nozzle/throttler v0.0.0-20180817012639-2ea982251481 // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector v0.125.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector v0.125.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opencensusexporter v0.125.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/syslogexporter v0.125.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/zipkinexporter v0.125.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.125.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil v0.125.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent v0.125.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/core/xidutils v0.125.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden v0.125.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl v0.125.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.125.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza v0.125.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger v0.125.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus v0.125.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin v0.125.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver v0.125.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/opencensusreceiver v0.125.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/syslogreceiver v0.125.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver v0.125.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/openzipkin/zipkin-go v0.4.3 // indirect
	github.com/pelletier/go-toml/v2 v2.2.2 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/prometheus/client_golang v1.22.0 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.62.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/sagikazarmark/locafero v0.4.0 // indirect
	github.com/sagikazarmark/slog-shim v0.1.0 // indirect
	github.com/sassoftware/relic v7.2.1+incompatible // indirect
	github.com/secure-systems-lab/go-securesystemslib v0.9.0 // indirect
	github.com/segmentio/ksuid v1.0.4 // indirect
	github.com/shibumi/go-pathspec v1.3.0 // indirect
	github.com/shirou/gopsutil/v4 v4.25.3 // indirect
	github.com/sigstore/fulcio v1.5.1 // indirect
	github.com/sigstore/protobuf-specs v0.3.2 // indirect
	github.com/sigstore/rekor v1.3.6 // indirect
	github.com/sigstore/sigstore v1.8.8 // indirect
	github.com/sigstore/timestamp-authority v1.2.2 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/skratchdot/open-golang v0.0.0-20200116055534-eef842397966 // indirect
	github.com/soheilhy/cmux v0.1.5 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spf13/afero v1.11.0 // indirect
	github.com/spf13/cast v1.6.0 // indirect
	github.com/spf13/cobra v1.9.1 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/spf13/viper v1.19.0 // indirect
	github.com/subosito/gotenv v1.6.0 // indirect
	github.com/syndtr/goleveldb v1.0.1-0.20220721030215-126854af5e6d // indirect
	github.com/thales-e-security/pool v0.0.2 // indirect
	github.com/theupdateframework/go-tuf v0.7.0 // indirect
	github.com/titanous/rocacheck v0.0.0-20171023193734-afe73141d399 // indirect
	github.com/tjfoc/gmsm v1.4.1 // indirect
	github.com/tklauser/go-sysconf v0.3.14 // indirect
	github.com/tklauser/numcpus v0.8.0 // indirect
	github.com/transparency-dev/merkle v0.0.2 // indirect
	github.com/twmb/murmur3 v1.1.8 // indirect
	github.com/ua-parser/uap-go v0.0.0-20240611065828-3a4781585db6 // indirect
	github.com/valyala/fastjson v1.6.4 // indirect
	github.com/vbatts/tar-split v0.11.5 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/xanzy/go-gitlab v0.107.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.mongodb.org/mongo-driver v1.14.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/client v1.31.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/component/componentstatus v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/component/componenttest v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/config/configauth v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/config/configcompression v1.31.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/config/configgrpc v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/config/confighttp v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/config/configmiddleware v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/config/confignet v1.31.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/config/configretry v1.31.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/confmap/xconfmap v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/connector v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/connector/connectortest v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/connector/xconnector v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/consumer v1.31.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/consumer/consumererror/xconsumererror v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/consumer/consumertest v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/exporter v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/exporter/debugexporter v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/exporter/exporterhelper/xexporterhelper v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/exporter/exportertest v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/exporter/otlpexporter v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/exporter/otlphttpexporter v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/exporter/xexporter v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/extension v1.31.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/extension/extensionauth v1.31.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/extension/extensioncapabilities v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/extension/extensionmiddleware v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/extension/extensiontest v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/extension/xextension v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/extension/zpagesextension v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/featuregate v1.31.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/internal/fanoutconsumer v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/internal/memorylimiter v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/internal/sharedcomponent v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/otelcol v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/pdata/testdata v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/pipeline v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/processor v1.31.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/processor/batchprocessor v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/processor/memorylimiterprocessor v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/processor/processorhelper v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/processor/processorhelper/xprocessorhelper v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/processor/processortest v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/processor/xprocessor v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/receiver v1.31.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/receiver/otlpreceiver v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/receiver/receiverhelper v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/receiver/receivertest v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/collector/service/hostcapabilities v0.125.1-0.20250430101735-2ecd0b7f95cd // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.60.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.60.0 // indirect
	go.opentelemetry.io/contrib/propagators/b3 v1.35.0 // indirect
	go.opentelemetry.io/contrib/zpages v0.60.0 // indirect
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
	go.opentelemetry.io/otel/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk/log v0.11.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.35.0 // indirect
	go.opentelemetry.io/proto/otlp v1.5.0 // indirect
	go.step.sm/crypto v0.51.1 // indirect
	golang.org/x/crypto v0.37.0 // indirect
	golang.org/x/exp v0.0.0-20250210185358-939b2ce775ac // indirect
	golang.org/x/mod v0.23.0 // indirect
	golang.org/x/net v0.39.0 // indirect
	golang.org/x/oauth2 v0.28.0 // indirect
	golang.org/x/sync v0.13.0 // indirect
	golang.org/x/term v0.31.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	golang.org/x/time v0.11.0 // indirect
	gonum.org/v1/gonum v0.16.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250313205543-e70fdf4c4cb4 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250313205543-e70fdf4c4cb4 // indirect
	google.golang.org/grpc v1.72.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gotest.tools/v3 v3.1.0 // indirect
	k8s.io/api v0.31.1 // indirect
	k8s.io/apimachinery v0.31.4 // indirect
	k8s.io/client-go v0.31.1 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/kube-openapi v0.0.0-20240228011516-70dd3763d340 // indirect
	k8s.io/utils v0.0.0-20240711033017-18e509b52bc8 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/release-utils v0.8.4 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

replace go.opentelemetry.io/collector/extension/extensionauth => go.opentelemetry.io/collector/extension/extensionauth v0.0.0-20250226024140-8099e51f9a77

replace go.opentelemetry.io/collector/extension/extensionauth/extensionauthtest => go.opentelemetry.io/collector/extension/extensionauth/extensionauthtest v0.0.0-20250226024140-8099e51f9a77

replace go.opentelemetry.io/collector/service/hostcapabilities => go.opentelemetry.io/collector/service/hostcapabilities v0.0.0-20250226024140-8099e51f9a77

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/syslogreceiver => ../../receiver/syslogreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza => ../../pkg/stanza

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage => ../../extension/storage

replace github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector => ../../connector/routingconnector

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver => ../../receiver/signalfxreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite => ../../pkg/translator/prometheusremotewrite

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter => ../../exporter/prometheusexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus => ../../pkg/translator/prometheus

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/zipkinexporter => ../../exporter/zipkinexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/syslogexporter => ../../exporter/syslogexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr => ../../pkg/batchperresourceattr

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter => ../../exporter/sapmexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin => ../../pkg/translator/zipkin

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter => ../../exporter/splunkhecexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver => ../../receiver/prometheusreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger => ../../pkg/translator/jaeger

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil => ../../internal/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata => ../../pkg/experimentalmetricmetadata

replace github.com/open-telemetry/opentelemetry-collector-contrib/testbed/mockdatasenders/mockdatadogagentexporter => ../../testbed/mockdatasenders/mockdatadogagentexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/carbonexporter => ../../exporter/carbonexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry => ../../pkg/resourcetotelemetry

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver => ../../receiver/zipkinreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/opencensusreceiver => ../../receiver/opencensusreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent => ../../internal/sharedcomponent

replace github.com/open-telemetry/opentelemetry-collector-contrib/testbed => ../../testbed

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver => ../../receiver/sapmreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver => ../../receiver/datadogreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver => ../../receiver/carbonreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus => ../../pkg/translator/opencensus

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/core/xidutils => ../../pkg/core/xidutils

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opencensusexporter => ../../exporter/opencensusexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk => ../../internal/splunk

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver => ../../receiver/splunkhecreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter => ../../exporter/signalfxexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver => ../../receiver/jaegerreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl => ../../pkg/ottl

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector => ../../connector/spanmetricsconnector

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/signalfx => ../../pkg/translator/signalfx

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics => ../../internal/exp/metrics

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/ackextension => ../../extension/ackextension

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter => ../../exporter/prometheusremotewriteexporter
