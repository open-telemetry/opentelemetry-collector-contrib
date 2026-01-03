module github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor

go 1.24.0

require (
	cloud.google.com/go/compute/metadata v0.9.0
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v1.30.0
	github.com/aws/aws-sdk-go-v2 v1.41.0
	github.com/aws/aws-sdk-go-v2/config v1.32.6
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.16
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.279.0
	github.com/cenkalti/backoff/v5 v5.0.3
	github.com/digitalocean/go-metadata v0.0.0-20250129100319-e3650a3df44b
	github.com/google/go-cmp v0.7.0
	github.com/hashicorp/consul/api v1.32.1
	github.com/hetznercloud/hcloud-go/v2 v2.33.0
	github.com/linode/go-metadata v0.2.3
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil v0.142.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.142.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig v0.142.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders v0.142.0
	github.com/scaleway/scaleway-sdk-go v1.0.0-beta.36
	github.com/shirou/gopsutil/v4 v4.25.11
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/component v1.48.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/component/componenttest v0.142.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/config/confighttp v0.142.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/config/configopaque v1.48.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/config/configtls v1.48.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/confmap v1.48.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/confmap/xconfmap v0.142.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/consumer v1.48.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/consumer/consumertest v0.142.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/consumer/xconsumer v0.142.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/featuregate v1.48.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/pdata v1.48.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/pdata/pprofile v0.142.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/processor v1.48.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/processor/processorhelper v0.142.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/processor/processorhelper/xprocessorhelper v0.142.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/processor/processortest v0.142.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/collector/processor/xprocessor v0.142.1-0.20251231114627-ad49dc64648d
	go.opentelemetry.io/otel v1.39.0
	go.uber.org/goleak v1.3.0
	go.uber.org/multierr v1.11.0
	go.uber.org/zap v1.27.1
)

require (
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/Showmax/go-fqdn v1.0.0 // indirect
	github.com/armon/go-metrics v0.4.1 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.19.6 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.16 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.16 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.16 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.0.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.30.8 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.35.12 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.41.5 // indirect
	github.com/aws/smithy-go v1.24.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/docker v28.5.2+incompatible // indirect
	github.com/docker/go-connections v0.5.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/ebitengine/purego v0.9.1 // indirect
	github.com/emicklei/go-restful/v3 v3.12.2 // indirect
	github.com/fatih/color v1.16.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20250903184740-5d135037bd4d // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-resty/resty/v2 v2.17.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/gnostic-models v0.7.0 // indirect
	github.com/google/go-tpm v0.9.8 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-hclog v1.5.0 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-version v1.8.0 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/hashicorp/serf v0.10.1 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.18.2 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.5.1-0.20231216201459-8508981c8b6c // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/sys/sequential v0.6.0 // indirect
	github.com/moby/term v0.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0-rc5 // indirect
	github.com/openshift/api v0.0.0-20251015095338-264e80a2b6e7 // indirect
	github.com/openshift/client-go v0.0.0-20251015124057-db0dee36e235 // indirect
	github.com/pierrec/lz4/v4 v4.1.23 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/prometheus/client_golang v1.23.2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.66.1 // indirect
	github.com/prometheus/procfs v0.16.1 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/tklauser/go-sysconf v0.3.16 // indirect
	github.com/tklauser/numcpus v0.11.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/client v1.48.1-0.20251231114627-ad49dc64648d // indirect
	go.opentelemetry.io/collector/component/componentstatus v0.142.1-0.20251231114627-ad49dc64648d // indirect
	go.opentelemetry.io/collector/config/configauth v1.48.1-0.20251231114627-ad49dc64648d // indirect
	go.opentelemetry.io/collector/config/configcompression v1.48.1-0.20251231114627-ad49dc64648d // indirect
	go.opentelemetry.io/collector/config/configmiddleware v1.48.1-0.20251231114627-ad49dc64648d // indirect
	go.opentelemetry.io/collector/config/configoptional v1.48.1-0.20251231114627-ad49dc64648d // indirect
	go.opentelemetry.io/collector/extension/extensionauth v1.48.1-0.20251231114627-ad49dc64648d // indirect
	go.opentelemetry.io/collector/extension/extensionmiddleware v0.142.1-0.20251231114627-ad49dc64648d // indirect
	go.opentelemetry.io/collector/pdata/testdata v0.142.1-0.20251231114627-ad49dc64648d // indirect
	go.opentelemetry.io/collector/pipeline v1.48.1-0.20251231114627-ad49dc64648d // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.63.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.30.0 // indirect
	go.opentelemetry.io/otel/metric v1.39.0 // indirect
	go.opentelemetry.io/otel/sdk v1.39.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.39.0 // indirect
	go.opentelemetry.io/otel/trace v1.39.0 // indirect
	go.opentelemetry.io/proto/otlp v1.3.1 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.46.0 // indirect
	golang.org/x/exp v0.0.0-20250305212735-054e65f0b394 // indirect
	golang.org/x/mod v0.30.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/oauth2 v0.32.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/term v0.38.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	golang.org/x/time v0.12.0 // indirect
	golang.org/x/tools v0.39.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251029180050-ab9386a59fda // indirect
	google.golang.org/grpc v1.78.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.12.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/api v0.34.3 // indirect
	k8s.io/apimachinery v0.34.3 // indirect
	k8s.io/client-go v0.34.3 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/kube-openapi v0.0.0-20250710124328-f3f2b991d03b // indirect
	k8s.io/utils v0.0.0-20250604170112-4c0f3b243397 // indirect
	sigs.k8s.io/json v0.0.0-20241014173422-cfa47c3a1cc8 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.3.0 // indirect
	sigs.k8s.io/yaml v1.6.0 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil => ../../internal/aws/ecsutil

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders => ../../internal/metadataproviders

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig => ../../internal/k8sconfig
