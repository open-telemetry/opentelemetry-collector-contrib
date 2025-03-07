module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefareceiver

go 1.23.0

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/bearertokenauthextension v0.121.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver v0.121.0
	github.com/prometheus/common v0.62.0
	github.com/prometheus/prometheus v0.300.1
	github.com/stretchr/testify v1.10.0
	go.opentelemetry.io/collector/component v1.27.1-0.20250307164521-7c787571daa5
	go.opentelemetry.io/collector/component/componenttest v0.121.1-0.20250307145831-dc9250a6c150
	go.opentelemetry.io/collector/config/configauth v0.121.1-0.20250307145831-dc9250a6c150
	go.opentelemetry.io/collector/config/confighttp v0.121.1-0.20250307145831-dc9250a6c150
	go.opentelemetry.io/collector/config/configtls v1.27.1-0.20250307164521-7c787571daa5
	go.opentelemetry.io/collector/confmap v1.27.1-0.20250307164521-7c787571daa5
	go.opentelemetry.io/collector/confmap/xconfmap v0.121.1-0.20250307164521-7c787571daa5
	go.opentelemetry.io/collector/consumer v1.27.1-0.20250307164521-7c787571daa5
	go.opentelemetry.io/collector/consumer/consumertest v0.121.1-0.20250307145831-dc9250a6c150
	go.opentelemetry.io/collector/extension/extensiontest v0.121.1-0.20250307145831-dc9250a6c150
	go.opentelemetry.io/collector/pipeline v0.121.1-0.20250307145831-dc9250a6c150
	go.opentelemetry.io/collector/receiver v0.121.1-0.20250307145831-dc9250a6c150
	go.opentelemetry.io/collector/receiver/receivertest v0.121.1-0.20250307145831-dc9250a6c150
	go.uber.org/goleak v1.3.0
	go.uber.org/multierr v1.11.0
)

require (
	cloud.google.com/go/auth v0.9.5 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.4 // indirect
	cloud.google.com/go/compute/metadata v0.6.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.14.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.7.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.10.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5 v5.7.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4 v4.3.0 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.2.2 // indirect
	github.com/Code-Hex/go-generics-cache v1.5.1 // indirect
	github.com/Microsoft/go-winio v0.6.1 // indirect
	github.com/alecthomas/units v0.0.0-20240626203959-61d1e3462e30 // indirect
	github.com/armon/go-metrics v0.4.1 // indirect
	github.com/aws/aws-sdk-go v1.55.5 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cncf/xds/go v0.0.0-20241223141626-cff3c89139a3 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dennwc/varint v1.0.0 // indirect
	github.com/digitalocean/godo v1.126.0 // indirect
	github.com/distribution/reference v0.5.0 // indirect
	github.com/docker/docker v27.3.1+incompatible // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect
	github.com/envoyproxy/go-control-plane/envoy v1.32.4 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.2.1 // indirect
	github.com/fatih/color v1.16.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.8.0 // indirect
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-openapi/jsonpointer v0.20.2 // indirect
	github.com/go-openapi/jsonreference v0.20.4 // indirect
	github.com/go-openapi/swag v0.22.9 // indirect
	github.com/go-resty/resty/v2 v2.13.1 // indirect
	github.com/go-viper/mapstructure/v2 v2.2.1 // indirect
	github.com/go-zookeeper/zk v1.0.4 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.2.1 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/gnostic-models v0.6.8 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/s2a-go v0.1.8 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.4 // indirect
	github.com/googleapis/gax-go/v2 v2.13.0 // indirect
	github.com/gophercloud/gophercloud v1.14.1 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/grafana/regexp v0.0.0-20240518133315-a468a5bfb3bc // indirect
	github.com/hashicorp/consul/api v1.29.4 // indirect
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
	github.com/hashicorp/nomad/api v0.0.0-20240717122358-3d93bd3778f3 // indirect
	github.com/hashicorp/serf v0.10.1 // indirect
	github.com/hetznercloud/hcloud-go/v2 v2.13.1 // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/ionos-cloud/sdk-go/v6 v6.2.1 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/knadh/koanf/maps v0.1.1 // indirect
	github.com/knadh/koanf/providers/confmap v0.1.0 // indirect
	github.com/knadh/koanf/v2 v2.1.2 // indirect
	github.com/kolo/xmlrpc v0.0.0-20220921171641-a4b6fa1dd06b // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/linode/linodego v1.41.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/miekg/dns v1.1.62 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.5.1-0.20231216201459-8508981c8b6c // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.121.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus v0.121.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.2 // indirect
	github.com/ovh/go-ovh v1.6.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.22 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_golang v1.21.1 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common/sigv4 v0.1.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/scaleway/scaleway-sdk-go v1.0.0-beta.30 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/vultr/govultr/v2 v2.17.2 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/collector/client v1.27.1-0.20250307164521-7c787571daa5 // indirect
	go.opentelemetry.io/collector/component/componentstatus v0.121.1-0.20250307145831-dc9250a6c150 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.27.1-0.20250307164521-7c787571daa5 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.27.1-0.20250307164521-7c787571daa5 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.121.1-0.20250307145831-dc9250a6c150 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.121.1-0.20250307145831-dc9250a6c150 // indirect
	go.opentelemetry.io/collector/extension v1.27.1-0.20250307164521-7c787571daa5 // indirect
	go.opentelemetry.io/collector/extension/extensionauth v0.121.1-0.20250307145831-dc9250a6c150 // indirect
	go.opentelemetry.io/collector/featuregate v1.27.1-0.20250307164521-7c787571daa5 // indirect
	go.opentelemetry.io/collector/pdata v1.27.1-0.20250307164521-7c787571daa5 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.121.1-0.20250307145831-dc9250a6c150 // indirect
	go.opentelemetry.io/collector/receiver/receiverhelper v0.0.0-20250307145831-dc9250a6c150 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.121.1-0.20250307145831-dc9250a6c150 // indirect
	go.opentelemetry.io/collector/semconv v0.121.1-0.20250307145831-dc9250a6c150 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.60.0 // indirect
	go.opentelemetry.io/otel v1.35.0 // indirect
	go.opentelemetry.io/otel/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk v1.35.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.35.0 // indirect
	go.opentelemetry.io/otel/trace v1.35.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	go.uber.org/zap/exp v0.3.0 // indirect
	golang.org/x/crypto v0.35.0 // indirect
	golang.org/x/exp v0.0.0-20240506185415-9bf2ced13842 // indirect
	golang.org/x/mod v0.21.0 // indirect
	golang.org/x/net v0.36.0 // indirect
	golang.org/x/oauth2 v0.26.0 // indirect
	golang.org/x/sync v0.11.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/term v0.29.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	golang.org/x/time v0.6.0 // indirect
	golang.org/x/tools v0.26.0 // indirect
	google.golang.org/api v0.199.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250218202821-56aae31c358a // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250218202821-56aae31c358a // indirect
	google.golang.org/grpc v1.71.0 // indirect
	google.golang.org/protobuf v1.36.5 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/api v0.31.1 // indirect
	k8s.io/apimachinery v0.31.1 // indirect
	k8s.io/client-go v0.31.1 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/kube-openapi v0.0.0-20240228011516-70dd3763d340 // indirect
	k8s.io/utils v0.0.0-20240711033017-18e509b52bc8 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver => ../prometheusreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/bearertokenauthextension => ../../extension/bearertokenauthextension

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus => ../../pkg/translator/prometheus

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite => ../../pkg/translator/prometheusremotewrite

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry => ../../pkg/resourcetotelemetry

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter => ../../exporter/prometheusremotewriteexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../pkg/pdatautil

retract (
	v0.76.2
	v0.76.1
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../pkg/golden
