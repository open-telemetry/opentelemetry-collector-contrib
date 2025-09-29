module github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog

go 1.24.0

require (
	github.com/DataDog/datadog-agent/comp/otelcol/otlp/testutil v0.72.0-devel
	github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/inframetadata v0.72.0-devel
	github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes v0.72.0-devel
	github.com/DataDog/datadog-agent/pkg/util/hostname/validate v0.72.0-devel
	github.com/DataDog/datadog-api-client-go/v2 v2.46.0
	github.com/DataDog/gohai v0.0.0-20230524154621-4316413895ee
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v1.29.0
	github.com/aws/aws-sdk-go-v2 v1.37.0
	github.com/aws/aws-sdk-go-v2/config v1.30.1
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.0
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.237.0
	github.com/cenkalti/backoff/v4 v4.3.0
	github.com/google/go-cmp v0.7.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil v0.136.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig v0.136.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders v0.136.0
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/component v1.42.1-0.20250925151503-069408608b28
	go.opentelemetry.io/collector/component/componenttest v0.136.1-0.20250925151503-069408608b28
	go.opentelemetry.io/collector/config/configcompression v1.42.1-0.20250925151503-069408608b28
	go.opentelemetry.io/collector/config/confighttp v0.136.1-0.20250925151503-069408608b28
	go.opentelemetry.io/collector/config/configretry v1.42.1-0.20250925151503-069408608b28
	go.opentelemetry.io/collector/config/configtls v1.42.1-0.20250925151503-069408608b28
	go.opentelemetry.io/collector/consumer/consumererror v0.136.1-0.20250925151503-069408608b28
	go.opentelemetry.io/collector/exporter v1.42.1-0.20250925151503-069408608b28
	go.opentelemetry.io/collector/exporter/exportertest v0.136.1-0.20250925151503-069408608b28
	go.opentelemetry.io/collector/pdata v1.42.1-0.20250925151503-069408608b28
	go.opentelemetry.io/otel v1.38.0
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.0
	k8s.io/apimachinery v0.32.3
	k8s.io/client-go v0.32.3
)

require (
	cloud.google.com/go/compute/metadata v0.7.0 // indirect
	github.com/DataDog/datadog-agent/comp/core/secrets/def v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/comp/core/secrets/mock v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/comp/core/secrets/utils v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/comp/core/tagger/origindetection v0.62.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/comp/core/tagger/types v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/comp/core/tagger/utils v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/collector/check/defaults v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/config/create v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/config/env v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/config/mock v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/config/model v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/config/nodetreemodel v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/config/setup v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/config/structure v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/config/teeconfig v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/config/viperconfig v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/fips v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/proto v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/serializer v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/util/executable v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/util/filesystem v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/util/log v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/util/option v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/util/pointer v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/util/scrubber v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/util/system v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/util/system/socket v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/util/winutil v0.72.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/version v0.72.0-devel // indirect
	github.com/DataDog/sketches-go v1.4.7 // indirect
	github.com/DataDog/viper v1.14.1-0.20250612143030-1b15c8822ed4 // indirect
	github.com/DataDog/zstd v1.5.6 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.18.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.0 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.0 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.26.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.31.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.35.0 // indirect
	github.com/aws/smithy-go v1.22.5 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cihub/seelog v0.0.0-20170130134532-f561c5e57575 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/ebitengine/purego v0.8.4 // indirect
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20250903184740-5d135037bd4d // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/gofrs/flock v0.12.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/go-tpm v0.9.6 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/hashicorp/hcl v1.0.1-vault-7 // indirect
	github.com/hectane/go-acl v0.0.0-20230122075934-ca0b05cb1adb // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20250317134145-8bc96cf8fc35 // indirect
	github.com/magiconair/properties v1.8.10 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/mapstructure v1.5.1-0.20231216201459-8508981c8b6c // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.136.0 // indirect
	github.com/openshift/api v3.9.0+incompatible // indirect
	github.com/openshift/client-go v0.0.0-20241203091221-452dfb8fa071 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/philhofer/fwd v1.1.3-0.20240916144458-20a13a1f6b7c // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/shirou/gopsutil/v3 v3.24.4 // indirect
	github.com/shirou/gopsutil/v4 v4.25.8 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/spf13/afero v1.14.0 // indirect
	github.com/spf13/cast v1.9.2 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/tinylib/msgp v1.3.0 // indirect
	github.com/tklauser/go-sysconf v0.3.15 // indirect
	github.com/tklauser/numcpus v0.10.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/client v1.42.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/config/configauth v0.136.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/config/configmiddleware v1.42.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.42.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/config/configoptional v0.136.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/confmap v1.42.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/confmap/xconfmap v0.136.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/consumer v1.42.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/consumer/consumertest v0.136.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.136.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/exporter/xexporter v0.136.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/extension/extensionauth v1.42.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/extension/extensionmiddleware v0.136.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/featuregate v1.42.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.136.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.136.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/pipeline v1.42.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/receiver v1.42.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/receiver/receivertest v0.136.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.136.1-0.20250925151503-069408608b28 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.13.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.63.0 // indirect
	go.opentelemetry.io/otel/log v0.14.0 // indirect
	go.opentelemetry.io/otel/metric v1.38.0 // indirect
	go.opentelemetry.io/otel/sdk v1.38.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.38.0 // indirect
	go.opentelemetry.io/otel/trace v1.38.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.41.0 // indirect
	golang.org/x/exp v0.0.0-20250819193227-8b4c13bb791b // indirect
	golang.org/x/net v0.43.0 // indirect
	golang.org/x/oauth2 v0.30.0 // indirect
	golang.org/x/sys v0.35.0 // indirect
	golang.org/x/term v0.34.0 // indirect
	golang.org/x/text v0.28.0 // indirect
	golang.org/x/time v0.12.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250707201910-8d1bb00bc6a7 // indirect
	google.golang.org/grpc v1.75.1 // indirect
	google.golang.org/protobuf v1.36.9 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.12.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/api v0.32.3 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/kube-openapi v0.0.0-20241105132330-32ad38e42d3f // indirect
	k8s.io/utils v0.0.0-20241104100929-3ea5e8cea738 // indirect
	sigs.k8s.io/json v0.0.0-20241010143419-9aa6b5e7a4b3 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.5.0 // indirect
	sigs.k8s.io/yaml v1.5.0 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../common

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders => ../metadataproviders

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig => ../k8sconfig

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil => ../aws/ecsutil

// openshift removed all tags from their repo, use the pseudoversion from the release-3.9 branch HEAD
replace github.com/openshift/api v3.9.0+incompatible => github.com/openshift/api v0.0.0-20180801171038-322a19404e37
