module github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog

go 1.25.0

require (
	github.com/DataDog/datadog-agent/comp/otelcol/otlp/testutil v0.80.2
	github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/inframetadata v0.83.0-devel.0.20260714134811-fee4bbf7ff73
	github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes v0.83.0-devel.0.20260714134811-fee4bbf7ff73
	github.com/DataDog/datadog-agent/pkg/util/hostname/validate v0.80.2
	github.com/DataDog/datadog-api-client-go/v2 v2.62.0
	github.com/DataDog/gohai v0.0.0-20230524154621-4316413895ee
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v1.34.0
	github.com/aws/aws-sdk-go-v2 v1.42.1
	github.com/aws/aws-sdk-go-v2/config v1.32.29
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.30
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.316.0
	github.com/cenkalti/backoff/v7 v7.0.0
	github.com/google/go-cmp v0.7.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil v0.156.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig v0.156.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders v0.156.0
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/component v1.62.1-0.20260720221237-c82363844ef5
	go.opentelemetry.io/collector/component/componenttest v0.156.1-0.20260720221237-c82363844ef5
	go.opentelemetry.io/collector/config/configcompression v1.62.1-0.20260720221237-c82363844ef5
	go.opentelemetry.io/collector/config/confighttp v0.156.1-0.20260720221237-c82363844ef5
	go.opentelemetry.io/collector/config/configretry v1.62.1-0.20260720221237-c82363844ef5
	go.opentelemetry.io/collector/config/configtls v1.62.1-0.20260720221237-c82363844ef5
	go.opentelemetry.io/collector/consumer/consumererror v0.156.1-0.20260720221237-c82363844ef5
	go.opentelemetry.io/collector/exporter v1.62.1-0.20260720221237-c82363844ef5
	go.opentelemetry.io/collector/exporter/exportertest v0.156.1-0.20260720221237-c82363844ef5
	go.opentelemetry.io/collector/pdata v1.62.1-0.20260720221237-c82363844ef5
	go.opentelemetry.io/otel v1.44.1-0.20260622141720-fbe3d073ba93
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.28.0
	k8s.io/apimachinery v0.35.4
	k8s.io/client-go v0.35.4
)

require (
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	github.com/DataDog/datadog-agent/comp/core/delegatedauth v0.80.2 // indirect
	github.com/DataDog/datadog-agent/comp/core/secrets/def v0.80.2 // indirect
	github.com/DataDog/datadog-agent/comp/core/tagger/origindetection v0.80.2 // indirect
	github.com/DataDog/datadog-agent/comp/core/tagger/types v0.80.2 // indirect
	github.com/DataDog/datadog-agent/comp/core/tagger/utils v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/collector/check/defaults v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/config/basic v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/config/buildschema v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/config/create v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/config/env v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/config/helper v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/config/mock v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/config/model v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/config/nodetreemodel v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/config/setup v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/config/structure v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/config/teeconfig v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/config/viperconfig v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/fips v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/proto v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/serializer v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/template v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/trace/log v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/trace/traceutil v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/util/executable v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/util/filesystem v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/util/log v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/util/pointer v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/util/scrubber v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/util/system v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/util/winutil v0.82.0-devel.0.20260624113434-509b872045c2 // indirect
	github.com/DataDog/datadog-agent/pkg/version v0.80.2 // indirect
	github.com/DataDog/go-acl v1.0.1 // indirect
	github.com/DataDog/sketches-go v1.4.8 // indirect
	github.com/DataDog/viper v1.15.1 // indirect
	github.com/DataDog/zstd v1.5.8-0.20260421145859-31a7e515a571 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.19.28 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.30 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.30 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.31 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.30 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.4.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.32.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.37.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.44.0 // indirect
	github.com/aws/smithy-go v1.27.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cihub/seelog v0.0.0-20170130134532-f561c5e57575 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/ebitengine/purego v0.10.0 // indirect
	github.com/emicklei/go-restful/v3 v3.12.2 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20251226215517-609e4778396f // indirect
	github.com/fsnotify/fsnotify v1.10.1 // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/gofrs/flock v0.13.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/gnostic-models v0.7.0 // indirect
	github.com/google/go-tpm v0.9.8 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.19.0 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.5 // indirect
	github.com/lufia/plan9stats v0.0.0-20260216142805-b3301c5f2a88 // indirect
	github.com/magiconair/properties v1.8.10 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mdlayher/socket v0.6.0 // indirect
	github.com/mdlayher/vsock v1.3.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.156.0 // indirect
	github.com/openshift/api v0.0.0-20251015095338-264e80a2b6e7 // indirect
	github.com/openshift/client-go v0.0.0-20251015124057-db0dee36e235 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/philhofer/fwd v1.2.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.27 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/shirou/gopsutil/v3 v3.24.4 // indirect
	github.com/shirou/gopsutil/v4 v4.26.6 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/stretchr/objx v0.5.3 // indirect
	github.com/tinylib/msgp v1.6.4 // indirect
	github.com/tklauser/go-sysconf v0.3.16 // indirect
	github.com/tklauser/numcpus v0.11.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/client v1.62.1-0.20260720221237-c82363844ef5 // indirect
	go.opentelemetry.io/collector/config/configauth v1.62.1-0.20260720221237-c82363844ef5 // indirect
	go.opentelemetry.io/collector/config/configmiddleware v1.62.1-0.20260720221237-c82363844ef5 // indirect
	go.opentelemetry.io/collector/config/confignet v1.62.1-0.20260720221237-c82363844ef5 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.62.1-0.20260720221237-c82363844ef5 // indirect
	go.opentelemetry.io/collector/config/configoptional v1.62.1-0.20260720221237-c82363844ef5 // indirect
	go.opentelemetry.io/collector/confmap v1.62.1-0.20260720221237-c82363844ef5 // indirect
	go.opentelemetry.io/collector/consumer v1.62.1-0.20260720221237-c82363844ef5 // indirect
	go.opentelemetry.io/collector/consumer/consumertest v0.156.1-0.20260720221237-c82363844ef5 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.156.1-0.20260720221237-c82363844ef5 // indirect
	go.opentelemetry.io/collector/exporter/xexporter v0.156.1-0.20260720221237-c82363844ef5 // indirect
	go.opentelemetry.io/collector/extension/extensionauth v1.62.1-0.20260720221237-c82363844ef5 // indirect
	go.opentelemetry.io/collector/extension/extensionmiddleware v0.156.1-0.20260720221237-c82363844ef5 // indirect
	go.opentelemetry.io/collector/featuregate v1.62.1-0.20260720221237-c82363844ef5 // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.156.1-0.20260720221237-c82363844ef5 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.156.1-0.20260720221237-c82363844ef5 // indirect
	go.opentelemetry.io/collector/pipeline v1.62.1-0.20260720221237-c82363844ef5 // indirect
	go.opentelemetry.io/collector/receiver v1.62.1-0.20260720221237-c82363844ef5 // indirect
	go.opentelemetry.io/collector/receiver/receivertest v0.156.1-0.20260720221237-c82363844ef5 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.156.1-0.20260720221237-c82363844ef5 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.69.0 // indirect
	go.opentelemetry.io/otel/metric v1.44.1-0.20260622141720-fbe3d073ba93 // indirect
	go.opentelemetry.io/otel/sdk v1.44.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.44.1-0.20260622141720-fbe3d073ba93 // indirect
	go.opentelemetry.io/otel/trace v1.44.1-0.20260622141720-fbe3d073ba93 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.54.0 // indirect
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/oauth2 v0.36.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/term v0.45.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
	google.golang.org/grpc v1.82.0 // indirect
	google.golang.org/protobuf v1.36.12-0.20260116114154-8c4c4ae446ca // indirect
	gopkg.in/evanphx/json-patch.v4 v4.13.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/api v0.35.4 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/kube-openapi v0.0.0-20250910181357-589584f1c912 // indirect
	k8s.io/utils v0.0.0-20251002143259-bc988d571ff4 // indirect
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.3.0 // indirect
	sigs.k8s.io/yaml v1.6.0 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../common

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders => ../metadataproviders

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig => ../k8sconfig

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil => ../aws/ecsutil
