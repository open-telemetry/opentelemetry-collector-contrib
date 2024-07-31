module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter

go 1.21.0

require (
	github.com/DataDog/agent-payload/v5 v5.0.127
	github.com/DataDog/datadog-agent/comp/core/config v0.56.0-rc.7
	github.com/DataDog/datadog-agent/comp/core/hostname/hostnameinterface v0.56.0-rc.7
	github.com/DataDog/datadog-agent/comp/core/log v0.56.0-rc.7
	github.com/DataDog/datadog-agent/comp/logs/agent/config v0.56.0-rc.7
	github.com/DataDog/datadog-agent/comp/otelcol/logsagentpipeline v0.56.0-rc.7
	github.com/DataDog/datadog-agent/comp/otelcol/logsagentpipeline/logsagentpipelineimpl v0.56.0-rc.7
	github.com/DataDog/datadog-agent/comp/otelcol/otlp/components/exporter/logsagentexporter v0.56.0-rc.7
	github.com/DataDog/datadog-agent/comp/otelcol/otlp/components/metricsclient v0.56.0-rc.7
	github.com/DataDog/datadog-agent/comp/otelcol/otlp/testutil v0.57.0-devel.0.20240718200853-81bf3b2e412d
	github.com/DataDog/datadog-agent/comp/trace/compression/impl-gzip v0.56.0-rc.7
	github.com/DataDog/datadog-agent/pkg/config/model v0.56.0-rc.7
	github.com/DataDog/datadog-agent/pkg/config/setup v0.56.0-rc.7
	github.com/DataDog/datadog-agent/pkg/logs/auditor v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/client v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/diagnostic v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/pipeline v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/sds v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/sources v0.56.0-rc.7
	github.com/DataDog/datadog-agent/pkg/logs/status/statusinterface v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/pkg/proto v0.56.0-rc.7
	github.com/DataDog/datadog-agent/pkg/status/health v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/pkg/trace v0.57.0-devel.0.20240722160158-ad956a31a730
	github.com/DataDog/datadog-agent/pkg/util/hostname/validate v0.56.0-rc.7
	github.com/DataDog/datadog-agent/pkg/util/startstop v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-api-client-go/v2 v2.28.0
	github.com/DataDog/datadog-go/v5 v5.5.0
	github.com/DataDog/gohai v0.0.0-20230524154621-4316413895ee
	github.com/DataDog/opentelemetry-mapping-go/pkg/inframetadata v0.17.0
	github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/attributes v0.17.0
	github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/logs v0.17.0
	github.com/DataDog/opentelemetry-mapping-go/pkg/otlp/metrics v0.17.0
	github.com/DataDog/opentelemetry-mapping-go/pkg/quantile v0.17.0
	github.com/DataDog/sketches-go v1.4.6 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v1.24.1
	github.com/aws/aws-sdk-go v1.53.16
	github.com/cenkalti/backoff/v4 v4.3.0
	github.com/google/go-cmp v0.6.0
	github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector v0.106.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil v0.106.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.106.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig v0.106.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders v0.106.0
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry v0.106.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor v0.106.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor v0.106.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor v0.106.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor v0.106.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver v0.106.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver v0.106.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver v0.106.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver v0.106.0
	github.com/stretchr/testify v1.9.0
	go.opentelemetry.io/collector/component v0.106.1
	go.opentelemetry.io/collector/config/configauth v0.106.1
	go.opentelemetry.io/collector/config/configcompression v1.12.0
	go.opentelemetry.io/collector/config/confighttp v0.106.1
	go.opentelemetry.io/collector/config/confignet v0.106.1
	go.opentelemetry.io/collector/config/configopaque v1.12.0
	go.opentelemetry.io/collector/config/configretry v1.12.0
	go.opentelemetry.io/collector/config/configtls v1.12.0
	go.opentelemetry.io/collector/confmap v0.106.1
	go.opentelemetry.io/collector/connector v0.106.1
	go.opentelemetry.io/collector/consumer v0.106.1
	go.opentelemetry.io/collector/exporter v0.106.1
	go.opentelemetry.io/collector/featuregate v1.12.0
	go.opentelemetry.io/collector/otelcol v0.106.1
	go.opentelemetry.io/collector/otelcol/otelcoltest v0.106.1
	go.opentelemetry.io/collector/pdata v1.12.0
	go.opentelemetry.io/collector/processor v0.106.1
	go.opentelemetry.io/collector/processor/batchprocessor v0.106.1
	go.opentelemetry.io/collector/processor/memorylimiterprocessor v0.106.1
	go.opentelemetry.io/collector/receiver v0.106.1
	go.opentelemetry.io/collector/receiver/otlpreceiver v0.106.1
	go.opentelemetry.io/collector/semconv v0.106.1
	go.opentelemetry.io/otel/metric v1.28.0
	go.opentelemetry.io/otel/trace v1.28.0
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/goleak v1.3.0
	go.uber.org/zap v1.27.0
	google.golang.org/protobuf v1.34.2
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/zorkian/go-datadog-api.v2 v2.30.0
	k8s.io/apimachinery v0.29.3
	k8s.io/client-go v0.29.3
)

require (
	cloud.google.com/go/auth v0.5.1 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.2 // indirect
	cloud.google.com/go/compute/metadata v0.5.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.11.1 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.6.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.8.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5 v5.7.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4 v4.3.0 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.2.2 // indirect
	github.com/Code-Hex/go-generics-cache v1.5.1 // indirect
	github.com/DataDog/datadog-agent/comp/core/flare/builder v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/comp/core/flare/types v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/comp/core/secrets v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/comp/core/telemetry v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/comp/def v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/comp/otelcol/otlp/components/statsprocessor v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/comp/trace/compression/def v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/pkg/collector/check/defaults v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/pkg/config/env v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/pkg/config/utils v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/message v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/metrics v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/processor v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/sender v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/status/utils v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/pkg/obfuscate v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/pkg/remoteconfig/state v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/pkg/telemetry v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/pkg/util/backoff v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/pkg/util/cgroups v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/pkg/util/executable v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/pkg/util/filesystem v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/pkg/util/fxutil v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/pkg/util/http v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/pkg/util/log v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/pkg/util/optional v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/pkg/util/pointer v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/pkg/util/scrubber v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/pkg/util/statstracker v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/pkg/util/system v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/pkg/util/system/socket v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/pkg/util/winutil v0.56.0-rc.7 // indirect
	github.com/DataDog/datadog-agent/pkg/version v0.56.0-rc.7 // indirect
	github.com/DataDog/dd-sensitive-data-scanner/sds-go/go v0.0.0-20240419161837-f1b2f553edfe // indirect
	github.com/DataDog/go-sqllexer v0.0.12 // indirect
	github.com/DataDog/go-tuf v1.1.0-0.5.2 // indirect
	github.com/DataDog/viper v1.13.5 // indirect
	github.com/DataDog/zstd v1.5.5 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/Showmax/go-fqdn v1.0.0 // indirect
	github.com/alecthomas/participle/v2 v2.1.1 // indirect
	github.com/alecthomas/units v0.0.0-20231202071711-9a357b53e9c9 // indirect
	github.com/armon/go-metrics v0.4.1 // indirect
	github.com/benbjohnson/clock v1.3.5 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bmatcuk/doublestar/v4 v4.6.1 // indirect
	github.com/briandowns/spinner v1.23.0 // indirect
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cihub/seelog v0.0.0-20170130134532-f561c5e57575 // indirect
	github.com/cncf/xds/go v0.0.0-20240423153145-555b57ec207b // indirect
	github.com/containerd/cgroups/v3 v3.0.3 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dennwc/varint v1.0.0 // indirect
	github.com/digitalocean/godo v1.117.0 // indirect
	github.com/distribution/reference v0.5.0 // indirect
	github.com/docker/docker v26.1.5+incompatible // indirect
	github.com/docker/go-connections v0.5.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect
	github.com/envoyproxy/go-control-plane v0.12.0 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.0.4 // indirect
	github.com/expr-lang/expr v1.16.9 // indirect
	github.com/fatih/color v1.16.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-openapi/jsonpointer v0.20.2 // indirect
	github.com/go-openapi/jsonreference v0.20.4 // indirect
	github.com/go-openapi/swag v0.22.9 // indirect
	github.com/go-resty/resty/v2 v2.13.1 // indirect
	github.com/go-viper/mapstructure/v2 v2.0.0 // indirect
	github.com/go-zookeeper/zk v1.0.3 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/goccy/go-json v0.10.3 // indirect
	github.com/godbus/dbus/v5 v5.0.6 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.2.1 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/mock v1.6.0 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/s2a-go v0.1.7 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.2 // indirect
	github.com/googleapis/gax-go/v2 v2.12.4 // indirect
	github.com/gophercloud/gophercloud v1.12.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/grafana/regexp v0.0.0-20240518133315-a468a5bfb3bc // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.20.0 // indirect
	github.com/hashicorp/consul/api v1.29.2 // indirect
	github.com/hashicorp/cronexpr v1.1.2 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-hclog v1.6.3 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.7 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-version v1.7.0 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/hashicorp/nomad/api v0.0.0-20240604134157-e73d8bb1140d // indirect
	github.com/hashicorp/serf v0.10.1 // indirect
	github.com/hectane/go-acl v0.0.0-20190604041725-da78bae5fc95 // indirect
	github.com/hetznercloud/hcloud-go/v2 v2.9.0 // indirect
	github.com/iancoleman/strcase v0.3.0 // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/ionos-cloud/sdk-go/v6 v6.1.11 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/jonboulle/clockwork v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kardianos/osext v0.0.0-20190222173326-2bc1f35cddc0 // indirect
	github.com/karrick/godirwalk v1.17.0 // indirect
	github.com/klauspost/compress v1.17.9 // indirect
	github.com/knadh/koanf/maps v0.1.1 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/knadh/koanf/v2 v2.1.1 // indirect
	github.com/kolo/xmlrpc v0.0.0-20220921171641-a4b6fa1dd06b // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/leodido/go-syslog/v4 v4.1.0 // indirect
	github.com/leodido/ragel-machinery v0.0.0-20190525184631-5f46317e436b // indirect
	github.com/linode/linodego v1.35.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20220913051719-115f729f3c8c // indirect
	github.com/magiconair/properties v1.8.7 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/miekg/dns v1.1.59 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.5.1-0.20231216201459-8508981c8b6c // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/mostynb/go-grpc-compression v1.2.3 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza v0.106.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus v0.106.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.0 // indirect
	github.com/opencontainers/runtime-spec v1.1.0-rc.3 // indirect
	github.com/openshift/api v3.9.0+incompatible // indirect
	github.com/openshift/client-go v0.0.0-20210521082421-73d9475a9142 // indirect
	github.com/outcaste-io/ristretto v0.2.1 // indirect
	github.com/ovh/go-ovh v1.5.1 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/philhofer/fwd v1.1.2 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20220216144756-c35f1ee13d7c // indirect
	github.com/prometheus-community/windows_exporter v0.25.1 // indirect
	github.com/prometheus/client_golang v1.19.1 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.55.0 // indirect
	github.com/prometheus/common/sigv4 v0.1.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/prometheus/prometheus v0.53.1 // indirect
	github.com/rs/cors v1.11.0 // indirect
	github.com/scaleway/scaleway-sdk-go v1.0.0-beta.27 // indirect
	github.com/secure-systems-lab/go-securesystemslib v0.7.0 // indirect
	github.com/shirou/gopsutil/v3 v3.24.5 // indirect
	github.com/shirou/gopsutil/v4 v4.24.6 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/spf13/afero v1.10.0 // indirect
	github.com/spf13/cast v1.5.1 // indirect
	github.com/spf13/cobra v1.8.1 // indirect
	github.com/spf13/jwalterweatherman v1.0.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/stormcat24/protodep v0.1.8 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/tinylib/msgp v1.1.9 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/valyala/fastjson v1.6.4 // indirect
	github.com/vultr/govultr/v2 v2.17.2 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	github.com/zorkian/go-datadog-api v2.30.0+incompatible // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/collector v0.106.1 // indirect
	go.opentelemetry.io/collector/client v0.106.1 // indirect
	go.opentelemetry.io/collector/config/configgrpc v0.106.1 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.106.1 // indirect
	go.opentelemetry.io/collector/config/internal v0.106.1 // indirect
	go.opentelemetry.io/collector/confmap/converter/expandconverter v0.106.1 // indirect
	go.opentelemetry.io/collector/confmap/provider/envprovider v0.106.1 // indirect
	go.opentelemetry.io/collector/confmap/provider/fileprovider v0.106.1 // indirect
	go.opentelemetry.io/collector/confmap/provider/httpprovider v0.106.1 // indirect
	go.opentelemetry.io/collector/confmap/provider/yamlprovider v0.106.1 // indirect
	go.opentelemetry.io/collector/consumer/consumerprofiles v0.106.1 // indirect
	go.opentelemetry.io/collector/consumer/consumertest v0.106.1 // indirect
	go.opentelemetry.io/collector/extension v0.106.1 // indirect
	go.opentelemetry.io/collector/extension/auth v0.106.1 // indirect
	go.opentelemetry.io/collector/filter v0.106.1 // indirect
	go.opentelemetry.io/collector/internal/globalgates v0.106.1 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.106.1 // indirect
	go.opentelemetry.io/collector/pdata/testdata v0.106.1 // indirect
	go.opentelemetry.io/collector/service v0.106.1 // indirect
	go.opentelemetry.io/contrib/config v0.8.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.53.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.53.0 // indirect
	go.opentelemetry.io/contrib/propagators/b3 v1.28.0 // indirect
	go.opentelemetry.io/otel v1.28.0 // indirect
	go.opentelemetry.io/otel/bridge/opencensus v1.28.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.4.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.28.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.28.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.28.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.28.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.28.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.50.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v1.28.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.28.0 // indirect
	go.opentelemetry.io/otel/log v0.4.0 // indirect
	go.opentelemetry.io/otel/sdk v1.28.0 // indirect
	go.opentelemetry.io/otel/sdk/log v0.4.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.28.0 // indirect
	go.opentelemetry.io/proto/otlp v1.3.1 // indirect
	go.uber.org/dig v1.17.1 // indirect
	go.uber.org/fx v1.18.2 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/crypto v0.25.0 // indirect
	golang.org/x/exp v0.0.0-20240506185415-9bf2ced13842 // indirect
	golang.org/x/mod v0.18.0 // indirect
	golang.org/x/net v0.27.0 // indirect
	golang.org/x/oauth2 v0.21.0 // indirect
	golang.org/x/sync v0.7.0 // indirect
	golang.org/x/sys v0.22.0 // indirect
	golang.org/x/term v0.22.0 // indirect
	golang.org/x/text v0.16.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	golang.org/x/tools v0.22.0 // indirect
	gonum.org/v1/gonum v0.15.0 // indirect
	google.golang.org/api v0.183.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240701130421-f6361c86f094 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240701130421-f6361c86f094 // indirect
	google.golang.org/grpc v1.65.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/api v0.29.3 // indirect
	k8s.io/klog/v2 v2.120.1 // indirect
	k8s.io/kube-openapi v0.0.0-20240228011516-70dd3763d340 // indirect
	k8s.io/utils v0.0.0-20230726121419-3b25d923346b // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
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

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza => ../../pkg/stanza

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter => ../../internal/filter

retract (
	v0.76.2
	v0.76.1
	v0.65.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl => ../../pkg/ottl

// see https://github.com/DataDog/agent-payload/issues/218
exclude github.com/DataDog/agent-payload/v5 v5.0.59

// openshift removed all tags from their repo, use the pseudoversion from the release-3.9 branch HEAD
replace github.com/openshift/api v3.9.0+incompatible => github.com/openshift/api v0.0.0-20180801171038-322a19404e37

// It appears that the v0.2.0 tag was modified.  Replacing with v0.2.1
replace github.com/outcaste-io/ristretto v0.2.0 => github.com/outcaste-io/ristretto v0.2.1

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8stest => ../../internal/k8stest

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver => ../../receiver/dockerstatsreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker => ../../internal/docker

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden

replace github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector => ../../connector/datadogconnector

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus => ../../pkg/translator/prometheus

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter => ../prometheusremotewriteexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor => ../../processor/probabilisticsamplerprocessor

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver => ../../receiver/prometheusreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite => ../../pkg/translator/prometheusremotewrite

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor => ../../processor/tailsamplingprocessor

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage => ../../extension/storage

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor => ../../processor/transformprocessor

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling => ../../pkg/sampling

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil => ../../internal/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata => ../../pkg/experimentalmetricmetadata
