module github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog/e2e

go 1.24.0

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector v0.145.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter v0.145.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor v0.145.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor v0.145.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor v0.145.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor v0.145.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver v0.145.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver v0.145.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver v0.145.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver v0.145.0
	github.com/stretchr/testify v1.11.1
	go.opentelemetry.io/collector/connector v0.145.1-0.20260205185216-81bc641f26c0
	go.opentelemetry.io/collector/exporter v1.51.1-0.20260205185216-81bc641f26c0
	go.opentelemetry.io/collector/otelcol v0.145.1-0.20260205185216-81bc641f26c0
	go.opentelemetry.io/collector/otelcol/otelcoltest v0.145.1-0.20260205185216-81bc641f26c0
	go.opentelemetry.io/collector/processor v1.51.1-0.20260205185216-81bc641f26c0
	go.opentelemetry.io/collector/processor/batchprocessor v0.145.1-0.20260205185216-81bc641f26c0
	go.opentelemetry.io/collector/processor/memorylimiterprocessor v0.145.1-0.20260205185216-81bc641f26c0
	go.opentelemetry.io/collector/receiver v1.51.1-0.20260205185216-81bc641f26c0
	go.opentelemetry.io/collector/receiver/otlpreceiver v0.145.1-0.20260205185216-81bc641f26c0
	go.opentelemetry.io/collector/service v0.145.1-0.20260205185216-81bc641f26c0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	cloud.google.com/go/auth v0.17.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute v1.54.0 // indirect
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.20.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.13.1 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.11.2 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5 v5.7.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4 v4.3.0 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.6.0 // indirect
	github.com/Code-Hex/go-generics-cache v1.5.1 // indirect
	github.com/DataDog/agent-payload/v5 v5.0.179 // indirect
	github.com/DataDog/datadog-agent/comp/api/api/def v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/comp/core/config v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/comp/core/flare/builder v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/comp/core/flare/types v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/comp/core/hostname/hostnameinterface v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/comp/core/log/def v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/comp/core/secrets/def v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/comp/core/secrets/noop-impl v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/comp/core/status v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/comp/core/tagger/def v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/comp/core/tagger/origindetection v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/comp/core/tagger/types v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/comp/core/tagger/utils v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/comp/core/telemetry v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/comp/def v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/comp/forwarder/defaultforwarder v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/comp/forwarder/orchestrator/orchestratorinterface v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/comp/logs/agent/config v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/comp/otelcol/logsagentpipeline v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/comp/otelcol/logsagentpipeline/logsagentpipelineimpl v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/comp/otelcol/otlp/components/exporter/logsagentexporter v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/comp/otelcol/otlp/components/exporter/serializerexporter v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/comp/otelcol/otlp/components/metricsclient v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/comp/serializer/logscompression v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/comp/serializer/metricscompression v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/comp/trace/compression/def v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/comp/trace/compression/impl-gzip v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/aggregator/ckey v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/api v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/collector/check/defaults v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/config/create v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/config/env v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/config/helper v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/config/mock v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/config/model v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/config/nodetreemodel v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/config/setup v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/config/structure v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/config/teeconfig v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/config/utils v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/config/viperconfig v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/fips v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/client v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/diagnostic v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/message v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/metrics v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/pipeline v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/processor v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/sender v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/sources v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/status/statusinterface v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/status/utils v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/types v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/metrics v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/obfuscate v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/inframetadata v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/logs v0.77.0-devel.0.20260204174209-7d83cf04c5ba // indirect
	github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/metrics v0.77.0-devel.0.20260206173154-cb2278cb7b51 // indirect
	github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/rum v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/orchestrator/model v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/orchestrator/util v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/process/util/api v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/proto v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/remoteconfig/state v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/serializer v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/status/health v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/tagger/types v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/tagset v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/telemetry v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/template v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/trace v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/trace/log v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/trace/otel v0.77.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/trace/stats v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/trace/traceutil v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/util/backoff v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/util/buf v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/util/cgroups v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/util/common v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/util/compression v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/util/executable v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/util/filesystem v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/util/fxutil v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/util/hostname/validate v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/util/http v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/util/json v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/util/log v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/util/option v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/util/otel v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/util/pointer v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/util/quantile v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/util/scrubber v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/util/sort v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/util/startstop v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/util/statstracker v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/util/system v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/util/system/socket v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/util/winutil v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-agent/pkg/version v0.76.0-rc.4 // indirect
	github.com/DataDog/datadog-api-client-go/v2 v2.54.0 // indirect
	github.com/DataDog/datadog-go/v5 v5.8.3 // indirect
	github.com/DataDog/go-sqllexer v0.1.12 // indirect
	github.com/DataDog/go-tuf v1.1.1-0.5.2 // indirect
	github.com/DataDog/gohai v0.0.0-20230524154621-4316413895ee // indirect
	github.com/DataDog/mmh3 v0.0.0-20210722141835-012dc69a9e49 // indirect
	github.com/DataDog/sketches-go v1.4.7 // indirect
	github.com/DataDog/viper v1.15.0 // indirect
	github.com/DataDog/zstd v1.5.7 // indirect
	github.com/DataDog/zstd_0 v0.0.0-20210310093942-586c1286621f // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v1.31.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/Showmax/go-fqdn v1.0.0 // indirect
	github.com/alecthomas/participle/v2 v2.1.4 // indirect
	github.com/alecthomas/units v0.0.0-20240927000941-0f3dac36c52b // indirect
	github.com/antchfx/xmlquery v1.5.0 // indirect
	github.com/antchfx/xpath v1.3.5 // indirect
	github.com/armon/go-metrics v0.4.1 // indirect
	github.com/aws/aws-sdk-go-v2 v1.41.1 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.32.7 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.19.7 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.17 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.17 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.17 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.286.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecs v1.70.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.4 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.17 // indirect
	github.com/aws/aws-sdk-go-v2/service/lightsail v1.50.10 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.0.5 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.30.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.35.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.41.6 // indirect
	github.com/aws/smithy-go v1.24.0 // indirect
	github.com/bboreham/go-loser v0.0.0-20230920113527-fcc2c21820a3 // indirect
	github.com/benbjohnson/clock v1.3.5 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bmatcuk/doublestar/v4 v4.10.0 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cihub/seelog v0.0.0-20170130134532-f561c5e57575 // indirect
	github.com/cncf/xds/go v0.0.0-20251022180443-0feb69152e9f // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/coreos/go-systemd/v22 v22.6.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dennwc/varint v1.0.0 // indirect
	github.com/digitalocean/go-metadata v0.0.0-20250129100319-e3650a3df44b // indirect
	github.com/digitalocean/godo v1.171.0 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/docker v28.5.2+incompatible // indirect
	github.com/docker/go-connections v0.6.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/ebitengine/purego v0.9.1 // indirect
	github.com/edsrzf/mmap-go v1.2.0 // indirect
	github.com/elastic/go-grok v0.3.1 // indirect
	github.com/elastic/lunes v0.2.0 // indirect
	github.com/emicklei/go-restful/v3 v3.12.2 // indirect
	github.com/envoyproxy/go-control-plane/envoy v1.36.0 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.3.0 // indirect
	github.com/expr-lang/expr v1.17.7 // indirect
	github.com/facette/natsort v0.0.0-20181210072756-2cd4dd1e2dcb // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20251226215517-609e4778396f // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logfmt/logfmt v0.6.0 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-openapi/analysis v0.24.1 // indirect
	github.com/go-openapi/errors v0.22.4 // indirect
	github.com/go-openapi/jsonpointer v0.22.1 // indirect
	github.com/go-openapi/jsonreference v0.21.3 // indirect
	github.com/go-openapi/loads v0.23.2 // indirect
	github.com/go-openapi/spec v0.22.1 // indirect
	github.com/go-openapi/strfmt v0.25.0 // indirect
	github.com/go-openapi/swag v0.25.4 // indirect
	github.com/go-openapi/swag/cmdutils v0.25.4 // indirect
	github.com/go-openapi/swag/conv v0.25.4 // indirect
	github.com/go-openapi/swag/fileutils v0.25.4 // indirect
	github.com/go-openapi/swag/jsonname v0.25.4 // indirect
	github.com/go-openapi/swag/jsonutils v0.25.4 // indirect
	github.com/go-openapi/swag/loading v0.25.4 // indirect
	github.com/go-openapi/swag/mangling v0.25.4 // indirect
	github.com/go-openapi/swag/netutils v0.25.4 // indirect
	github.com/go-openapi/swag/stringutils v0.25.4 // indirect
	github.com/go-openapi/swag/typeutils v0.25.4 // indirect
	github.com/go-openapi/swag/yamlutils v0.25.4 // indirect
	github.com/go-openapi/validate v0.25.1 // indirect
	github.com/go-resty/resty/v2 v2.17.1 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/go-zookeeper/zk v1.0.4 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/goccy/go-yaml v1.19.2 // indirect
	github.com/gofrs/flock v0.13.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/mock v1.7.0-rc.1 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/gnostic-models v0.7.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/go-tpm v0.9.8 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.7 // indirect
	github.com/googleapis/gax-go/v2 v2.15.0 // indirect
	github.com/gophercloud/gophercloud/v2 v2.9.0 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/gorilla/websocket v1.5.4-0.20250319132907-e064f32e3674 // indirect
	github.com/grafana/regexp v0.0.0-20250905093917-f7b3be9d1853 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.27.4 // indirect
	github.com/hashicorp/consul/api v1.32.1 // indirect
	github.com/hashicorp/cronexpr v1.1.3 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-hclog v1.6.3 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.7 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-version v1.8.0 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/hashicorp/nomad/api v0.0.0-20260106084653-e8f2200c7039 // indirect
	github.com/hashicorp/serf v0.10.1 // indirect
	github.com/hectane/go-acl v0.0.0-20230122075934-ca0b05cb1adb // indirect
	github.com/hetznercloud/hcloud-go/v2 v2.36.0 // indirect
	github.com/iancoleman/strcase v0.3.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/ionos-cloud/sdk-go/v6 v6.3.6 // indirect
	github.com/jonboulle/clockwork v0.5.0 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/julienschmidt/httprouter v1.3.0 // indirect
	github.com/klauspost/compress v1.18.3 // indirect
	github.com/klauspost/cpuid/v2 v2.2.10 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.2 // indirect
	github.com/kolo/xmlrpc v0.0.0-20220921171641-a4b6fa1dd06b // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/leodido/go-syslog/v4 v4.3.0 // indirect
	github.com/leodido/ragel-machinery v0.0.0-20190525184631-5f46317e436b // indirect
	github.com/lightstep/go-expohisto v1.0.0 // indirect
	github.com/linode/go-metadata v0.2.4 // indirect
	github.com/linode/linodego v1.63.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20251013123823-9fd1530e3ec3 // indirect
	github.com/magefile/mage v1.15.0 // indirect
	github.com/magiconair/properties v1.8.10 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mdlayher/socket v0.5.1 // indirect
	github.com/mdlayher/vsock v1.2.1 // indirect
	github.com/miekg/dns v1.1.69 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.5.1-0.20231216201459-8508981c8b6c // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/mostynb/go-grpc-compression v1.2.3 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/oklog/ulid/v2 v2.1.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil v0.145.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.145.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.145.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog v0.145.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker v0.145.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics v0.145.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter v0.145.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/gopsutilenv v0.145.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig v0.145.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders v0.145.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil v0.145.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog v0.145.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata v0.145.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl v0.145.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.145.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry v0.145.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling v0.145.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza v0.145.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus v0.145.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters v0.145.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor v0.145.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/openshift/api v0.0.0-20251015095338-264e80a2b6e7 // indirect
	github.com/openshift/client-go v0.0.0-20251015124057-db0dee36e235 // indirect
	github.com/outcaste-io/ristretto v0.2.3 // indirect
	github.com/ovh/go-ovh v1.9.0 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/philhofer/fwd v1.2.0 // indirect
	github.com/pierrec/lz4/v4 v4.1.25 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/prometheus/alertmanager v0.30.0 // indirect
	github.com/prometheus/client_golang v1.23.2 // indirect
	github.com/prometheus/client_golang/exp v0.0.0-20260101091701-2cd067eb23c9 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.67.5 // indirect
	github.com/prometheus/common/assets v0.2.0 // indirect
	github.com/prometheus/exporter-toolkit v0.15.1 // indirect
	github.com/prometheus/otlptranslator v1.0.0 // indirect
	github.com/prometheus/procfs v0.19.2 // indirect
	github.com/prometheus/prometheus v0.309.2-0.20260113170727-c7bc56cf6c8f // indirect
	github.com/prometheus/sigv4 v0.3.0 // indirect
	github.com/puzpuzpuz/xsync/v3 v3.5.1 // indirect
	github.com/richardartoul/molecule v1.0.1-0.20240531184615-7ca0df43c0b3 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/scaleway/scaleway-sdk-go v1.0.0-beta.36 // indirect
	github.com/secure-systems-lab/go-securesystemslib v0.9.0 // indirect
	github.com/shirou/gopsutil/v3 v3.24.5 // indirect
	github.com/shirou/gopsutil/v4 v4.26.1 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/shurcooL/httpfs v0.0.0-20230704072500-f1e31cf0ba5c // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/cobra v1.10.2 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/stackitcloud/stackit-sdk-go/core v0.20.1 // indirect
	github.com/stretchr/objx v0.5.2 // indirect
	github.com/tilinna/clock v1.1.0 // indirect
	github.com/tinylib/msgp v1.6.3 // indirect
	github.com/tklauser/go-sysconf v0.3.16 // indirect
	github.com/tklauser/numcpus v0.11.0 // indirect
	github.com/twmb/murmur3 v1.1.8 // indirect
	github.com/ua-parser/uap-go v0.0.0-20240611065828-3a4781585db6 // indirect
	github.com/valyala/fastjson v1.6.7 // indirect
	github.com/vultr/govultr/v2 v2.17.2 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	github.com/zeebo/xxh3 v1.1.0 // indirect
	go.mongodb.org/mongo-driver v1.17.6 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/client v1.51.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/component v1.51.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/component/componentstatus v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/component/componenttest v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/config/configauth v1.51.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.51.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/config/configgrpc v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/config/confighttp v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/config/configmiddleware v1.51.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/config/confignet v1.51.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.51.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/config/configoptional v1.51.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/config/configretry v1.51.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/config/configtls v1.51.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/confmap v1.51.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/confmap/provider/envprovider v1.51.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/confmap/provider/fileprovider v1.51.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/confmap/provider/httpprovider v1.51.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/confmap/provider/yamlprovider v1.51.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/confmap/xconfmap v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/connector/connectortest v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/connector/xconnector v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/consumer v1.51.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/consumer/consumertest v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/exporter/exporterhelper v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/exporter/exportertest v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/exporter/xexporter v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/extension v1.51.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/extension/extensionauth v1.51.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/extension/extensioncapabilities v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/extension/extensionmiddleware v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/extension/extensiontest v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/extension/xextension v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/featuregate v1.51.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/filter v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/internal/fanoutconsumer v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/internal/memorylimiter v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/internal/sharedcomponent v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/pdata v1.51.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/pdata/testdata v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/pdata/xpdata v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/pipeline v1.51.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/processor/processorhelper v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/processor/processorhelper/xprocessorhelper v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/processor/processortest v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/processor/xprocessor v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/receiver/receiverhelper v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/receiver/receivertest v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/scraper v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/scraper/scraperhelper v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/collector/semconv v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/service/hostcapabilities v0.145.1-0.20260205185216-81bc641f26c0 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.13.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.63.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace v0.64.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.65.0 // indirect
	go.opentelemetry.io/contrib/otelconf v0.18.0 // indirect
	go.opentelemetry.io/contrib/propagators/b3 v1.38.0 // indirect
	go.opentelemetry.io/otel v1.40.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc v0.14.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.14.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.39.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.39.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.39.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.39.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.39.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.60.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutlog v0.14.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v1.39.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.39.0 // indirect
	go.opentelemetry.io/otel/log v0.16.0 // indirect
	go.opentelemetry.io/otel/metric v1.40.0 // indirect
	go.opentelemetry.io/otel/sdk v1.40.0 // indirect
	go.opentelemetry.io/otel/sdk/log v0.14.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.40.0 // indirect
	go.opentelemetry.io/otel/trace v1.40.0 // indirect
	go.opentelemetry.io/proto/otlp v1.9.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/dig v1.19.0 // indirect
	go.uber.org/fx v1.24.0 // indirect
	go.uber.org/goleak v1.3.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.1 // indirect
	go.uber.org/zap/exp v0.3.0 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.47.0 // indirect
	golang.org/x/exp v0.0.0-20251209150349-8475f28825e9 // indirect
	golang.org/x/mod v0.31.0 // indirect
	golang.org/x/net v0.49.0 // indirect
	golang.org/x/oauth2 v0.34.0 // indirect
	golang.org/x/sync v0.19.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	golang.org/x/term v0.39.0 // indirect
	golang.org/x/text v0.33.0 // indirect
	golang.org/x/time v0.14.0 // indirect
	golang.org/x/tools v0.40.0 // indirect
	gonum.org/v1/gonum v0.17.0 // indirect
	google.golang.org/api v0.258.0 // indirect
	google.golang.org/genproto v0.0.0-20250603155806-513f23925822 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20251222181119-0a764e51fe1b // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251222181119-0a764e51fe1b // indirect
	google.golang.org/grpc v1.78.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.12.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.67.0 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/api v0.34.3 // indirect
	k8s.io/apimachinery v0.35.0-alpha.0 // indirect
	k8s.io/client-go v0.34.3 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/kube-openapi v0.0.0-20250710124328-f3f2b991d03b // indirect
	k8s.io/utils v0.0.0-20250604170112-4c0f3b243397 // indirect
	sigs.k8s.io/json v0.0.0-20241014173422-cfa47c3a1cc8 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.3.0 // indirect
	sigs.k8s.io/yaml v1.6.0 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor => ../../../processor/k8sattributesprocessor

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil => ../../pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/core/xidutils => ../../../pkg/core/xidutils

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor => ../../../processor/deltatocumulativeprocessor

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ../../../pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog => ../../datadog

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver => ../../../receiver/filelogreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver => ../../../receiver/dockerstatsreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor => ../../../processor/transformprocessor

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza => ../../../pkg/stanza

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders => ../../metadataproviders

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig => ../../k8sconfig

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter => ../../../exporter/datadogexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage => ../../../extension/storage

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl => ../../../pkg/ottl

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ../../../pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata => ../../../pkg/experimentalmetricmetadata

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker => ../../docker

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver => ../../../receiver/prometheusreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite => ../../../pkg/translator/prometheusremotewrite

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry => ../../../pkg/resourcetotelemetry

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus => ../../../pkg/translator/prometheus

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling => ../../../pkg/sampling

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ../../../pkg/golden

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog => ../../../pkg/datadog

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/gopsutilenv => ../../gopsutilenv

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics => ../../exp/metrics

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../common

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor => ../../../processor/resourcedetectionprocessor

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters => ../../../pkg/winperfcounters

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter => ../../filter

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil => ../../aws/ecsutil

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver => ../../../receiver/hostmetricsreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector => ../../../connector/datadogconnector

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor => ../../../processor/tailsamplingprocessor

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter => ../../../exporter/prometheusremotewriteexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor => ../../../processor/probabilisticsamplerprocessor

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest => ../../../pkg/xk8stest

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../coreinternal
