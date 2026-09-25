module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver

go 1.26.0

require (
	github.com/aws/aws-sdk-go-v2 v1.47.1
	github.com/aws/aws-sdk-go-v2/config v1.33.6
	github.com/aws/aws-sdk-go-v2/service/cloudwatch v1.73.0
	github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs v1.88.1
	github.com/aws/aws-sdk-go-v2/service/sts v1.51.1
	github.com/goccy/go-json v0.10.6
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/k8sleaderelector v0.162.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sleaderelectortest v0.162.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden v0.162.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.162.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza v0.162.0
	github.com/stretchr/testify v1.12.1
	go.opentelemetry.io/collector/component v1.68.1-0.20261006173156-20487f801913
	go.opentelemetry.io/collector/component/componenttest v0.162.1-0.20261006173156-20487f801913
	go.opentelemetry.io/collector/config/configoptional v1.68.1-0.20261006173156-20487f801913
	go.opentelemetry.io/collector/confmap v1.68.1-0.20261006173156-20487f801913
	go.opentelemetry.io/collector/consumer v1.68.1-0.20261006173156-20487f801913
	go.opentelemetry.io/collector/consumer/consumertest v0.162.1-0.20261006173156-20487f801913
	go.opentelemetry.io/collector/extension/xextension v0.162.1-0.20261006173156-20487f801913
	go.opentelemetry.io/collector/pdata v1.68.1-0.20261006173156-20487f801913
	go.opentelemetry.io/collector/receiver v1.68.1-0.20261006173156-20487f801913
	go.opentelemetry.io/collector/receiver/receivertest v0.162.1-0.20261006173156-20487f801913
	go.opentelemetry.io/collector/receiver/xreceiver v0.162.1-0.20261006173156-20487f801913
	go.opentelemetry.io/collector/scraper v0.162.1-0.20261006173156-20487f801913
	go.opentelemetry.io/collector/scraper/scraperhelper v0.162.1-0.20261006173156-20487f801913
	go.opentelemetry.io/otel v1.47.0
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.28.0
)

require (
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.20 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.20.6 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.20.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.5.4 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.8.4 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.5.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.19 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.14.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.10.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.38.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.43.1 // indirect
	github.com/aws/smithy-go v1.28.1 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/elastic/lunes v0.2.2 // indirect
	github.com/emicklei/go-restful/v3 v3.13.0 // indirect
	github.com/expr-lang/expr v1.17.8 // indirect
	github.com/fxamacker/cbor/v2 v2.9.1 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/jsonpointer v1.0.0 // indirect
	github.com/go-openapi/jsonreference v1.0.0 // indirect
	github.com/go-openapi/swag v0.27.1 // indirect
	github.com/go-openapi/swag/cmdutils v0.27.1 // indirect
	github.com/go-openapi/swag/conv v0.27.1 // indirect
	github.com/go-openapi/swag/fileutils v0.27.1 // indirect
	github.com/go-openapi/swag/jsonutils v0.27.1 // indirect
	github.com/go-openapi/swag/loading v0.27.1 // indirect
	github.com/go-openapi/swag/mangling v0.27.1 // indirect
	github.com/go-openapi/swag/netutils v0.27.1 // indirect
	github.com/go-openapi/swag/pools v0.27.1 // indirect
	github.com/go-openapi/swag/stringutils v0.27.1 // indirect
	github.com/go-openapi/swag/typeutils v0.27.1 // indirect
	github.com/go-openapi/swag/yamlutils v0.27.1 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/gnostic-models v0.7.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/knadh/koanf/maps v0.1.3 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.1 // indirect
	github.com/knadh/koanf/v2 v2.3.7 // indirect
	github.com/leodido/go-syslog/v4 v4.6.0 // indirect
	github.com/leodido/ragel-machinery v0.0.0-20190525184631-5f46317e436b // indirect
	github.com/magefile/mage v1.15.0 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.162.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig v0.162.0 // indirect
	github.com/openshift/api v0.0.0-20251015095338-264e80a2b6e7 // indirect
	github.com/openshift/client-go v0.0.0-20251015124057-db0dee36e235 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/stretchr/objx v0.5.3 // indirect
	github.com/valyala/fastjson v1.6.10 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector/config/configauth v1.68.1-0.20261006173156-20487f801913 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.162.1-0.20261006173156-20487f801913 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.162.1-0.20261006173156-20487f801913 // indirect
	go.opentelemetry.io/collector/extension v1.68.1-0.20261006173156-20487f801913 // indirect
	go.opentelemetry.io/collector/extension/extensionauth v1.68.1-0.20261006173156-20487f801913 // indirect
	go.opentelemetry.io/collector/featuregate v1.68.1-0.20261006173156-20487f801913 // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.162.1-0.20261006173156-20487f801913 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.162.1-0.20261006173156-20487f801913 // indirect
	go.opentelemetry.io/collector/pdata/xpdata v0.162.1-0.20261006173156-20487f801913 // indirect
	go.opentelemetry.io/collector/pipeline v1.68.1-0.20261006173156-20487f801913 // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.162.1-0.20261006173156-20487f801913 // indirect
	go.opentelemetry.io/collector/receiver/receiverhelper v0.162.1-0.20261006173156-20487f801913 // indirect
	go.opentelemetry.io/otel/log v1.47.0 // indirect
	go.opentelemetry.io/otel/metric v1.47.0 // indirect
	go.opentelemetry.io/otel/sdk v1.47.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.47.0 // indirect
	go.opentelemetry.io/otel/trace v1.47.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/net v0.58.0 // indirect
	golang.org/x/oauth2 v0.36.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/term v0.45.0 // indirect
	golang.org/x/text v0.42.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	gonum.org/v1/gonum v0.17.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260706201446-f0a921348800 // indirect
	google.golang.org/grpc v1.84.0 // indirect
	google.golang.org/protobuf v1.36.12 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.13.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/api v0.37.1 // indirect
	k8s.io/apimachinery v0.37.1 // indirect
	k8s.io/client-go v0.37.1 // indirect
	k8s.io/klog/v2 v2.140.0 // indirect
	k8s.io/kube-openapi v0.0.0-20260721132016-d427ff9ee9ad // indirect
	k8s.io/utils v0.0.0-20260626114624-be93311217bd // indirect
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.4.2 // indirect
	sigs.k8s.io/yaml v1.6.0 // indirect
)

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza => ../../pkg/stanza

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage => ../../extension/storage

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/semconvtest => ../../pkg/semconvtest

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/k8sleaderelector => ../../extension/k8sleaderelector

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sleaderelectortest => ../../internal/k8sleaderelectortest

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig => ../../internal/k8sconfig
