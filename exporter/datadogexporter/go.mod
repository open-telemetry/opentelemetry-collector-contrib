module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter

go 1.18

require (
	github.com/DataDog/agent-payload/v5 v5.0.70
	github.com/DataDog/datadog-agent/pkg/otlp/model v0.43.0-rc.3.0.20230201114415-fae4332beb98
	github.com/DataDog/datadog-agent/pkg/quantile v0.43.0-rc.3.0.20230201114415-fae4332beb98
	github.com/DataDog/datadog-agent/pkg/trace v0.43.0-rc.3.0.20230201114415-fae4332beb98
	github.com/DataDog/datadog-api-client-go/v2 v2.7.0
	github.com/DataDog/gohai v0.0.0-20220718130825-1776f9beb9cc
	github.com/DataDog/sketches-go v1.4.1
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v1.11.1
	github.com/aws/aws-sdk-go v1.44.196
	github.com/cenkalti/backoff/v4 v4.2.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil v0.70.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.70.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig v0.70.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders v0.70.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry v0.70.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor v0.70.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor v0.70.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver v0.70.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver v0.70.0
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/stretchr/testify v1.8.1
	go.opentelemetry.io/collector v0.70.1-0.20230207214007-faf5a578038f
	go.opentelemetry.io/collector/component v0.70.1-0.20230207214007-faf5a578038f
	go.opentelemetry.io/collector/confmap v0.70.1-0.20230207214007-faf5a578038f
	go.opentelemetry.io/collector/consumer v0.70.1-0.20230207214007-faf5a578038f
	go.opentelemetry.io/collector/featuregate v0.70.1-0.20230207214007-faf5a578038f
	go.opentelemetry.io/collector/pdata v1.0.0-rc4.0.20230207214007-faf5a578038f
	go.opentelemetry.io/collector/processor/batchprocessor v0.70.1-0.20230207214007-faf5a578038f
	go.opentelemetry.io/collector/receiver/otlpreceiver v0.70.1-0.20230207214007-faf5a578038f
	go.opentelemetry.io/collector/semconv v0.70.1-0.20230207214007-faf5a578038f
	go.uber.org/multierr v1.9.0
	go.uber.org/zap v1.24.0
	google.golang.org/protobuf v1.28.1
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/zorkian/go-datadog-api.v2 v2.30.0
	k8s.io/apimachinery v0.26.1
	k8s.io/client-go v0.26.1
)

require (
	cloud.google.com/go/compute v1.14.0 // indirect
	cloud.google.com/go/compute/metadata v0.2.3 // indirect
	contrib.go.opencensus.io/exporter/prometheus v0.4.2 // indirect
	github.com/DataDog/datadog-agent/pkg/obfuscate v0.43.0-rc.3.0.20230201114415-fae4332beb98 // indirect
	github.com/DataDog/datadog-agent/pkg/remoteconfig/state v0.43.0-rc.3.0.20230201114415-fae4332beb98 // indirect
	github.com/DataDog/datadog-agent/pkg/util/cgroups v0.43.0-rc.3.0.20230201114415-fae4332beb98 // indirect
	github.com/DataDog/datadog-agent/pkg/util/log v0.43.0-rc.3.0.20230201114415-fae4332beb98 // indirect
	github.com/DataDog/datadog-agent/pkg/util/pointer v0.43.0-rc.3 // indirect
	github.com/DataDog/datadog-agent/pkg/util/scrubber v0.43.0-rc.3.0.20230201114415-fae4332beb98 // indirect
	github.com/DataDog/datadog-go/v5 v5.1.1 // indirect
	github.com/DataDog/go-tuf v0.3.0--fix-localmeta-fork // indirect
	github.com/DataDog/zstd v1.5.0 // indirect
	github.com/Microsoft/go-winio v0.5.2 // indirect
	github.com/Showmax/go-fqdn v1.0.0 // indirect
	github.com/antonmedv/expr v1.12.0 // indirect
	github.com/armon/go-metrics v0.3.10 // indirect
	github.com/benbjohnson/clock v1.3.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bmatcuk/doublestar/v4 v4.6.0 // indirect
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/cihub/seelog v0.0.0-20170130134532-f561c5e57575 // indirect
	github.com/containerd/cgroups v1.0.4 // indirect
	github.com/coreos/go-systemd/v22 v22.3.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/docker/distribution v2.8.1+incompatible // indirect
	github.com/docker/docker v23.0.0+incompatible // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/emicklei/go-restful/v3 v3.9.0 // indirect
	github.com/fatih/color v1.13.0 // indirect
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logfmt/logfmt v0.5.1 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/jsonreference v0.20.0 // indirect
	github.com/go-openapi/swag v0.19.14 // indirect
	github.com/godbus/dbus/v5 v5.0.6 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/glog v1.0.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/mock v1.6.0 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/gnostic v0.5.7-v3refs // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/hashicorp/consul/api v1.18.0 // indirect
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-hclog v1.3.1 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/hashicorp/serf v0.10.1 // indirect
	github.com/imdario/mergo v0.3.11 // indirect
	github.com/inconshreveable/mousetrap v1.0.1 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/karrick/godirwalk v1.17.0 // indirect
	github.com/klauspost/compress v1.15.15 // indirect
	github.com/knadh/koanf v1.5.0 // indirect
	github.com/leoluk/perflib_exporter v0.2.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20220517141722-cf486979b281 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.12 // indirect
	github.com/mattn/go-isatty v0.0.14 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mostynb/go-grpc-compression v1.1.17 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/observiq/ctimefmt v1.0.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.70.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter v0.70.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza v0.70.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.2 // indirect
	github.com/opencontainers/runtime-spec v1.0.3-0.20210326190908-1c3f411f0417 // indirect
	github.com/openshift/api v0.0.0-20210521075222-e273a339932a // indirect
	github.com/openshift/client-go v0.0.0-20210521082421-73d9475a9142 // indirect
	github.com/outcaste-io/ristretto v0.2.0 // indirect
	github.com/philhofer/fwd v1.1.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/power-devops/perfstat v0.0.0-20220216144756-c35f1ee13d7c // indirect
	github.com/prometheus/client_golang v1.14.0 // indirect
	github.com/prometheus/client_model v0.3.0 // indirect
	github.com/prometheus/common v0.39.0 // indirect
	github.com/prometheus/procfs v0.8.0 // indirect
	github.com/prometheus/statsd_exporter v0.22.7 // indirect
	github.com/rs/cors v1.8.3 // indirect
	github.com/secure-systems-lab/go-securesystemslib v0.4.0 // indirect
	github.com/shirou/gopsutil/v3 v3.23.1 // indirect
	github.com/spf13/cobra v1.6.1 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/stretchr/objx v0.5.0 // indirect
	github.com/tinylib/msgp v1.1.8 // indirect
	github.com/tklauser/go-sysconf v0.3.11 // indirect
	github.com/tklauser/numcpus v0.6.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	github.com/zorkian/go-datadog-api v2.30.0+incompatible // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.38.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.38.0 // indirect
	go.opentelemetry.io/contrib/propagators/b3 v1.13.0 // indirect
	go.opentelemetry.io/otel v1.13.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.36.0 // indirect
	go.opentelemetry.io/otel/metric v0.36.0 // indirect
	go.opentelemetry.io/otel/sdk v1.13.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v0.36.0 // indirect
	go.opentelemetry.io/otel/trace v1.13.0 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	golang.org/x/net v0.5.0 // indirect
	golang.org/x/oauth2 v0.3.0 // indirect
	golang.org/x/sys v0.4.0 // indirect
	golang.org/x/term v0.4.0 // indirect
	golang.org/x/text v0.6.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	gonum.org/v1/gonum v0.12.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20221202195650-67e5cbc046fd // indirect
	google.golang.org/grpc v1.52.3 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/api v0.26.1 // indirect
	k8s.io/klog/v2 v2.80.1 // indirect
	k8s.io/kube-openapi v0.0.0-20221012153701-172d655c2280 // indirect
	k8s.io/utils v0.0.0-20221107191617-1a15be271d1d // indirect
	sigs.k8s.io/json v0.0.0-20220713155537-f223a00ba0e2 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig => ../../internal/k8sconfig

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders => ../../internal/metadataproviders

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry => ../../pkg/resourcetotelemetry

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil => ../../internal/aws/ecsutil

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor => ../../processor/k8sattributesprocessor

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor => ../../processor/resourcedetectionprocessor

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver => ../../receiver/hostmetricsreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver => ../../receiver/filelogreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus => ../../pkg/translator/opencensus

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza => ../../pkg/stanza

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage => ../../extension/storage

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter => ../../internal/filter

retract v0.65.0

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl => ../../pkg/ottl

// see https://github.com/DataDog/agent-payload/issues/218
exclude github.com/DataDog/agent-payload/v5 v5.0.59
