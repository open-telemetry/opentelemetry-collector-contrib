module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sobjectsreceiver

go 1.22.0

require (
	github.com/google/uuid v1.6.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig v0.113.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8stest v0.113.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden v0.113.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest v0.113.0
	github.com/stretchr/testify v1.9.0
	go.opentelemetry.io/collector/component v0.113.0
	go.opentelemetry.io/collector/confmap v1.19.0
	go.opentelemetry.io/collector/consumer v0.113.0
	go.opentelemetry.io/collector/consumer/consumertest v0.113.0
	go.opentelemetry.io/collector/pdata v1.19.0
	go.opentelemetry.io/collector/receiver v0.113.0
	go.opentelemetry.io/collector/receiver/otlpreceiver v0.113.0
	go.opentelemetry.io/collector/receiver/receivertest v0.113.0
	go.opentelemetry.io/collector/semconv v0.113.0
	go.uber.org/zap v1.27.0
	k8s.io/apimachinery v0.31.2
	k8s.io/client-go v0.31.2
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1 // indirect
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/distribution/reference v0.5.0 // indirect
	github.com/docker/docker v27.3.1+incompatible // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.22.4 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.17.11 // indirect
	github.com/knadh/koanf/maps v0.1.1 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/knadh/koanf/v2 v2.1.1 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mostynb/go-grpc-compression v1.2.3 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.113.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0-rc4 // indirect
	github.com/openshift/api v3.9.0+incompatible // indirect
	github.com/openshift/client-go v0.0.0-20210521082421-73d9475a9142 // indirect
	github.com/pierrec/lz4/v4 v4.1.21 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.opentelemetry.io/collector v0.113.0 // indirect
	go.opentelemetry.io/collector/client v1.19.0 // indirect
	go.opentelemetry.io/collector/component/componentstatus v0.113.0 // indirect
	go.opentelemetry.io/collector/config/configauth v0.113.0 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.19.0 // indirect
	go.opentelemetry.io/collector/config/configgrpc v0.113.0 // indirect
	go.opentelemetry.io/collector/config/confighttp v0.113.0 // indirect
	go.opentelemetry.io/collector/config/confignet v1.19.0 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.19.0 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.113.0 // indirect
	go.opentelemetry.io/collector/config/configtls v1.19.0 // indirect
	go.opentelemetry.io/collector/config/internal v0.113.0 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.113.0 // indirect
	go.opentelemetry.io/collector/consumer/consumerprofiles v0.113.0 // indirect
	go.opentelemetry.io/collector/extension v0.113.0 // indirect
	go.opentelemetry.io/collector/extension/auth v0.113.0 // indirect
	go.opentelemetry.io/collector/internal/sharedcomponent v0.113.0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.113.0 // indirect
	go.opentelemetry.io/collector/pipeline v0.113.0 // indirect
	go.opentelemetry.io/collector/receiver/receiverprofiles v0.113.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.56.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.56.0 // indirect
	go.opentelemetry.io/otel v1.31.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.30.0 // indirect
	go.opentelemetry.io/otel/metric v1.31.0 // indirect
	go.opentelemetry.io/otel/sdk v1.31.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.31.0 // indirect
	go.opentelemetry.io/otel/trace v1.31.0 // indirect
	go.opentelemetry.io/proto/otlp v1.3.1 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/mod v0.17.0 // indirect
	golang.org/x/net v0.30.0 // indirect
	golang.org/x/oauth2 v0.22.0 // indirect
	golang.org/x/sync v0.8.0 // indirect
	golang.org/x/sys v0.26.0 // indirect
	golang.org/x/term v0.25.0 // indirect
	golang.org/x/text v0.19.0 // indirect
	golang.org/x/time v0.4.0 // indirect
	golang.org/x/tools v0.21.1-0.20240508182429-e35e4ccd0d2d // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240822170219-fc7c04adadcd // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20241007155032-5fefd90f89a9 // indirect
	google.golang.org/grpc v1.67.1 // indirect
	google.golang.org/protobuf v1.35.1 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.12.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/api v0.31.2 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/kube-openapi v0.0.0-20240228011516-70dd3763d340 // indirect
	k8s.io/utils v0.0.0-20240711033017-18e509b52bc8 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig => ../../internal/k8sconfig

// openshift removed all tags from their repo, use the pseudoversion from the release-3.9 branch HEAD
replace github.com/openshift/api v3.9.0+incompatible => github.com/openshift/api v0.0.0-20180801171038-322a19404e37

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8stest => ../../internal/k8stest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden
