module github.com/open-telemetry/opentelemetry-collector-contrib

go 1.17

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudlogserviceexporter v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsprometheusremotewriteexporter v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/carbonexporter v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dynatraceexporter v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticexporter v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/f5cloudexporter v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudexporter v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombexporter v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/humioexporter v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/influxdbexporter v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/jaegerexporter v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logzioexporter v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/newrelicexporter v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opencensusexporter v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sentryexporter v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stackdriverexporter v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tanzuobservabilityexporter v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/zipkinexporter v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/bearertokenauthextension v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/fluentbitextension v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/httpforwarder v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/hostobserver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/oidcauthextension v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatorateprocessor v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricsgenerationprocessor v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/routingprocessor v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanmetricsprocessor v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanprocessor v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dotnetdiagnosticsreceiver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fluentforwardreceiver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/influxdbreceiver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/journaldreceiver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/opencensusreceiver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexecreceiver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/syslogreceiver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcplogreceiver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/udplogreceiver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/wavefrontreceiver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsperfcountersreceiver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver v0.37.1
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zookeeperreceiver v0.37.1
	github.com/prometheus/prometheus v1.8.2-0.20210621150501-ff58416a0b02
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.37.0
	golang.org/x/sys v0.0.0-20210927094055-39ccf1dd6fa6
)

require github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver v0.37.1

require (
	cloud.google.com/go v0.97.0 // indirect
	cloud.google.com/go/monitoring v0.1.0 // indirect
	cloud.google.com/go/trace v0.1.0 // indirect
	code.cloudfoundry.org/clock v1.0.0 // indirect
	contrib.go.opencensus.io/exporter/prometheus v0.4.0 // indirect
	contrib.go.opencensus.io/exporter/stackdriver v0.13.8 // indirect
	github.com/Azure/azure-sdk-for-go v55.2.0+incompatible // indirect
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest v0.11.19 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.14 // indirect
	github.com/Azure/go-autorest/autorest/date v0.3.0 // indirect
	github.com/Azure/go-autorest/autorest/to v0.4.0 // indirect
	github.com/Azure/go-autorest/autorest/validation v0.3.1 // indirect
	github.com/Azure/go-autorest/logger v0.2.1 // indirect
	github.com/Azure/go-autorest/tracing v0.6.0 // indirect
	github.com/DataDog/agent-payload v4.85.0+incompatible // indirect
	github.com/DataDog/datadog-agent/pkg/quantile v0.31.1 // indirect
	github.com/DataDog/datadog-agent/pkg/trace/exportable v0.0.0-20201016145401-4646cf596b02 // indirect
	github.com/DataDog/datadog-agent/pkg/util/log v0.0.0-20201009092105-58e18918b2db // indirect
	github.com/DataDog/datadog-go v4.4.0+incompatible // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v1.0.0-RC2 // indirect
	github.com/Microsoft/go-winio v0.5.0 // indirect
	github.com/Shopify/sarama v1.30.0 // indirect
	github.com/Showmax/go-fqdn v1.0.0 // indirect
	github.com/StackExchange/wmi v1.2.1 // indirect
	github.com/alecthomas/units v0.0.0-20210208195552-ff826a37aa15 // indirect
	github.com/aliyun/aliyun-log-go-sdk v0.1.22 // indirect
	github.com/antonmedv/expr v1.9.0 // indirect
	github.com/apache/thrift v0.15.0 // indirect
	github.com/armon/go-metrics v0.3.9 // indirect
	github.com/avast/retry-go v3.0.0+incompatible // indirect
	github.com/aws/aws-sdk-go v1.41.1 // indirect
	github.com/beeker1121/goque v2.1.0+incompatible // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bmatcuk/doublestar/v3 v3.0.0 // indirect
	github.com/caio/go-tdigest v3.1.0+incompatible // indirect
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/cenkalti/backoff/v4 v4.1.1 // indirect
	github.com/census-instrumentation/opencensus-proto v0.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/cihub/seelog v0.0.0-20170130134532-f561c5e57575 // indirect
	github.com/containerd/containerd v1.5.7 // indirect
	github.com/coreos/go-oidc v2.2.1+incompatible // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgraph-io/ristretto v0.1.0 // indirect
	github.com/digitalocean/godo v1.62.0 // indirect
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/docker v20.10.9+incompatible // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/dynatrace-oss/dynatrace-metric-utils-go v0.3.0 // indirect
	github.com/eapache/go-resiliency v1.2.0 // indirect
	github.com/eapache/go-xerial-snappy v0.0.0-20180814174437-776d5712da21 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/facebookgo/clock v0.0.0-20150410010913-600d898af40a // indirect
	github.com/facebookgo/limitgroup v0.0.0-20150612190941-6abd8d71ec01 // indirect
	github.com/facebookgo/muster v0.0.0-20150708232844-fd3d7953fd52 // indirect
	github.com/fatih/color v1.12.0 // indirect
	github.com/felixge/httpsnoop v1.0.2 // indirect
	github.com/form3tech-oss/jwt-go v3.2.3+incompatible // indirect
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/getsentry/sentry-go v0.11.0 // indirect
	github.com/go-kit/kit v0.11.0 // indirect
	github.com/go-kit/log v0.2.0 // indirect
	github.com/go-logfmt/logfmt v0.5.1 // indirect
	github.com/go-logr/logr v0.4.0 // indirect
	github.com/go-ole/go-ole v1.2.5 // indirect
	github.com/go-redis/redis/v7 v7.4.1 // indirect
	github.com/go-resty/resty/v2 v2.1.1-0.20191201195748-d7b97669fe48 // indirect
	github.com/go-stack/stack v1.8.0 // indirect
	github.com/go-zookeeper/zk v1.0.2 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/gofrs/uuid v4.0.0+incompatible // indirect
	github.com/gogo/googleapis v1.4.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/go-cmp v0.5.6 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/googleapis/gax-go/v2 v2.1.1 // indirect
	github.com/googleapis/gnostic v0.5.5 // indirect
	github.com/gophercloud/gophercloud v0.18.0 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.16.0 // indirect
	github.com/hashicorp/consul/api v1.10.1 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-hclog v0.16.2 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-uuid v1.0.2 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/hashicorp/serf v0.9.5 // indirect
	github.com/hetznercloud/hcloud-go v1.26.2 // indirect
	github.com/honeycombio/libhoney-go v1.15.5 // indirect
	github.com/iancoleman/strcase v0.2.0 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/influxdata/influxdb-observability/common v0.2.7 // indirect
	github.com/influxdata/influxdb-observability/influx2otel v0.0.0-20210927215436-943f6419116a // indirect
	github.com/influxdata/influxdb-observability/otel2influx v0.0.0-20210927215436-943f6419116a // indirect
	github.com/influxdata/line-protocol/v2 v2.0.0-20210520103755-6551a972d603 // indirect
	github.com/jaegertracing/jaeger v1.27.0 // indirect
	github.com/jcmturner/aescts/v2 v2.0.0 // indirect
	github.com/jcmturner/dnsutils/v2 v2.0.0 // indirect
	github.com/jcmturner/gofork v1.0.0 // indirect
	github.com/jcmturner/gokrb5/v8 v8.4.2 // indirect
	github.com/jcmturner/rpc/v2 v2.0.3 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51 // indirect
	github.com/klauspost/compress v1.13.6 // indirect
	github.com/knadh/koanf v1.2.4 // indirect
	github.com/leoluk/perflib_exporter v0.1.0 // indirect
	github.com/linode/linodego v0.28.5 // indirect
	github.com/logzio/jaeger-logzio v1.0.4 // indirect
	github.com/logzio/logzio-go v1.0.3 // indirect
	github.com/magiconair/properties v1.8.5 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.8 // indirect
	github.com/mattn/go-isatty v0.0.14 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/microsoft/ApplicationInsights-Go v0.4.4 // indirect
	github.com/miekg/dns v1.1.43 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.4.2 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mongodb-forks/digest v1.0.3 // indirect
	github.com/montanaflynn/stats v0.0.0-20171201202039-1bf9dbcd8cbe // indirect
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f // indirect
	github.com/newrelic/newrelic-telemetry-sdk-go v0.8.1 // indirect
	github.com/observiq/ctimefmt v1.0.0 // indirect
	github.com/observiq/go-syslog/v3 v3.0.2 // indirect
	github.com/olivere/elastic v6.2.37+incompatible // indirect
	github.com/onsi/ginkgo v1.16.4 // indirect
	github.com/onsi/gomega v1.14.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer v0.37.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil v0.37.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics v0.37.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy v0.37.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray v0.37.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.37.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker v0.37.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/interval v0.37.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig v0.37.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/kubelet v0.37.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk v0.37.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/stanza v0.37.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr v0.37.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal v0.37.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata v0.37.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry v0.37.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger v0.37.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus v0.37.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin v0.37.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/scraperhelper v0.37.1 // indirect
	github.com/open-telemetry/opentelemetry-log-collection v0.22.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.2-0.20190823105129-775207bd45b6 // indirect
	github.com/openlyinc/pointy v1.1.2 // indirect
	github.com/openshift/api v0.0.0-20210521075222-e273a339932a // indirect
	github.com/openshift/client-go v0.0.0-20210521082421-73d9475a9142 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/openzipkin/zipkin-go v0.2.5 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/pelletier/go-toml v1.9.4 // indirect
	github.com/philhofer/fwd v1.1.1 // indirect
	github.com/pierrec/lz4 v2.6.1+incompatible // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/pquerna/cachecontrol v0.1.0 // indirect
	github.com/prometheus/client_golang v1.11.0 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.31.1 // indirect
	github.com/prometheus/procfs v0.7.3 // indirect
	github.com/prometheus/statsd_exporter v0.21.0 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475 // indirect
	github.com/rs/cors v1.8.0 // indirect
	github.com/santhosh-tekuri/jsonschema v1.2.4 // indirect
	github.com/scaleway/scaleway-sdk-go v1.0.0-beta.7.0.20210223165440-c65ae3540d44 // indirect
	github.com/shirou/gopsutil v3.21.9+incompatible // indirect
	github.com/shirou/gopsutil/v3 v3.21.9 // indirect
	github.com/signalfx/com_signalfx_metrics_protobuf v0.0.2 // indirect
	github.com/signalfx/gohistogram v0.0.0-20160107210732-1ccfd2ff5083 // indirect
	github.com/signalfx/golib/v3 v3.3.13 // indirect
	github.com/signalfx/sapm-proto v0.7.0 // indirect
	github.com/signalfx/signalfx-agent/pkg/apm v0.0.0-20201202163743-65b4fa925fc8 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/soheilhy/cmux v0.1.5 // indirect
	github.com/spf13/afero v1.6.0 // indirect
	github.com/spf13/cast v1.4.1 // indirect
	github.com/spf13/cobra v1.2.1 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/spf13/viper v1.9.0 // indirect
	github.com/stretchr/objx v0.2.0 // indirect
	github.com/subosito/gotenv v1.2.0 // indirect
	github.com/syndtr/goleveldb v1.0.0 // indirect
	github.com/tidwall/gjson v1.8.1 // indirect
	github.com/tidwall/match v1.0.3 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	github.com/tidwall/tinylru v1.0.2 // indirect
	github.com/tidwall/wal v0.1.5 // indirect
	github.com/tinylib/msgp v1.1.6 // indirect
	github.com/tklauser/go-sysconf v0.3.9 // indirect
	github.com/tklauser/numcpus v0.3.0 // indirect
	github.com/uber/jaeger-client-go v2.29.1+incompatible // indirect
	github.com/uber/jaeger-lib v2.4.1+incompatible // indirect
	github.com/vmihailenco/msgpack/v5 v5.3.4 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	github.com/wavefronthq/wavefront-sdk-go v0.9.9 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.0.2 // indirect
	github.com/xdg-go/stringprep v1.0.2 // indirect
	go.elastic.co/apm v1.14.0 // indirect
	go.elastic.co/fastjson v1.1.0 // indirect
	go.etcd.io/bbolt v1.3.6 // indirect
	go.mongodb.org/atlas v0.12.0 // indirect
	go.opencensus.io v0.23.0 // indirect
	go.opentelemetry.io/collector/model v0.37.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.25.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.25.0 // indirect
	go.opentelemetry.io/contrib/zpages v0.25.0 // indirect
	go.opentelemetry.io/otel v1.0.1 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.24.0 // indirect
	go.opentelemetry.io/otel/internal/metric v0.24.0 // indirect
	go.opentelemetry.io/otel/metric v0.24.0 // indirect
	go.opentelemetry.io/otel/sdk v1.0.1 // indirect
	go.opentelemetry.io/otel/sdk/export/metric v0.24.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v0.24.0 // indirect
	go.opentelemetry.io/otel/trace v1.0.1 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.7.0 // indirect
	go.uber.org/zap v1.19.1 // indirect
	golang.org/x/crypto v0.0.0-20210920023735-84f357641f63 // indirect
	golang.org/x/net v0.0.0-20210927181540-4e4d966f7476 // indirect
	golang.org/x/oauth2 v0.0.0-20210819190943-2bc19b11175f // indirect
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c // indirect
	golang.org/x/term v0.0.0-20210220032956-6a3ed077a48d // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/time v0.0.0-20210723032227-1f47c861a9ac // indirect
	gonum.org/v1/gonum v0.9.3 // indirect
	google.golang.org/api v0.58.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20210927142257-433400c27d05 // indirect
	google.golang.org/grpc v1.41.0 // indirect
	google.golang.org/protobuf v1.27.1 // indirect
	gopkg.in/DataDog/dd-trace-go.v1 v1.33.0 // indirect
	gopkg.in/alexcesaro/statsd.v2 v2.0.0 // indirect
	gopkg.in/fsnotify/fsnotify.v1 v1.4.7 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.63.2 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
	gopkg.in/square/go-jose.v2 v2.5.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
	gopkg.in/zorkian/go-datadog-api.v2 v2.30.0 // indirect
	k8s.io/api v0.22.2 // indirect
	k8s.io/apimachinery v0.22.2 // indirect
	k8s.io/client-go v0.22.2 // indirect
	k8s.io/klog/v2 v2.9.0 // indirect
	k8s.io/kubelet v0.22.2 // indirect
	k8s.io/utils v0.0.0-20210819203725-bdf08cb9a70a // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.1.2 // indirect
	sigs.k8s.io/yaml v1.2.0 // indirect
)

// Replace references to modules that are in this repository with their relateive paths
// so that we always build with current (latest) version of the source code.

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ./internal/common

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ./internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk => ./internal/splunk

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig => ./internal/k8sconfig

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/kubelet => ./internal/kubelet

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics => ./internal/aws/metrics

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy => ./internal/aws/proxy

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray => ./internal/aws/xray

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil => ./internal/aws/awsutil

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/stanza => ./internal/stanza

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/interval => ./internal/interval

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker => ./internal/docker

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudlogserviceexporter => ./exporter/alibabacloudlogserviceexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsprometheusremotewriteexporter => ./exporter/awsprometheusremotewriteexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter => ./exporter/awsemfexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter => ./exporter/awsxrayexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter => ./exporter/azuremonitorexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/carbonexporter => ./exporter/carbonexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter => ./exporter/datadogexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dynatraceexporter => ./exporter/dynatraceexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/f5cloudexporter => ./exporter/f5cloudexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter => ./exporter/fileexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombexporter => ./exporter/honeycombexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/humioexporter => ./exporter/humioexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/influxdbexporter => ./exporter/influxdbexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/jaegerexporter => ./exporter/jaegerexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter => ./exporter/kafkaexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter => ./exporter/loadbalancingexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/newrelicexporter => ./exporter/newrelicexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter => ./exporter/awskinesisexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logzioexporter => ./exporter/logzioexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter => ./exporter/lokiexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opencensusexporter => ./exporter/opencensusexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter => ./exporter/prometheusexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter => ./exporter/prometheusremotewriteexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter => ./exporter/sapmexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sentryexporter => ./exporter/sentryexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter => ./exporter/signalfxexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter => ./exporter/splunkhecexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter => ./exporter/sumologicexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tanzuobservabilityexporter => ./exporter/tanzuobservabilityexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticexporter => ./exporter/elasticexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/zipkinexporter => ./exporter/zipkinexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/bearertokenauthextension => ./extension/bearertokenauthextension

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/fluentbitextension => ./extension/fluentbitextension

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension => ./extension/healthcheckextension

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/httpforwarder => ./extension/httpforwarder

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension => ./extension/oauth2clientauthextension

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer => ./extension/observer

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/hostobserver => ./extension/observer/hostobserver

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver => ./extension/observer/k8sobserver

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/oidcauthextension => ./extension/oidcauthextension

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension => ./extension/pprofextension

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage => ./extension/storage

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr => ./pkg/batchperresourceattr

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal => ./pkg/batchpersignal

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata => ./pkg/experimentalmetricmetadata

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry => ./pkg/resourcetotelemetry

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger => ./pkg/translator/jaeger

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus => ./pkg/translator/opencensus

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin => ./pkg/translator/zipkin

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver => ./receiver/awsecscontainermetricsreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver => ./receiver/awsxrayreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver => ./receiver/carbonreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudfoundryreceiver => ./receiver/cloudfoundryreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver => ./receiver/collectdreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dotnetdiagnosticsreceiver => ./receiver/dotnetdiagnosticsreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver => ./receiver/statsdreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver => ./receiver/kubeletstatsreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver => ./receiver/mongodbatlasreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver => ./receiver/redisreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator => ./receiver/receivercreator

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver => ./receiver/sapmreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver => ./receiver/k8sclusterreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver => ./receiver/signalfxreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver => ./receiver/splunkhecreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver => ./receiver/simpleprometheusreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/opencensusreceiver => ./receiver/opencensusreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexecreceiver => ./receiver/prometheusexecreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver => ./receiver/prometheusreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver => ./receiver/podmanreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/wavefrontreceiver => ./receiver/wavefrontreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver => ./receiver/dockerstatsreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsperfcountersreceiver => ./receiver/windowsperfcountersreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/influxdbreceiver => ./receiver/influxdbreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver => ./receiver/jaegerreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver => ./receiver/jmxreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/journaldreceiver => ./receiver/journaldreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zookeeperreceiver => ./receiver/zookeeperreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver => ./receiver/filelogreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fluentforwardreceiver => ./receiver/fluentforwardreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver => ./receiver/kafkareceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver => ./receiver/hostmetricsreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/scraperhelper => ./receiver/scraperhelper

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/syslogreceiver => ./receiver/syslogreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcplogreceiver => ./receiver/tcplogreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/udplogreceiver => ./receiver/udplogreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver => ./receiver/kafkametricsreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver => ./receiver/zipkinreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor => ./processor/attributesprocessor

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor => ./processor/filterprocessor

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor => ./processor/groupbyattrsprocessor

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor => ./processor/groupbytraceprocessor

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor => ./processor/k8sattributesprocessor/

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor => ./processor/resourcedetectionprocessor/

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor => ./processor/resourceprocessor/

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor => ./processor/metricstransformprocessor/

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricsgenerationprocessor => ./processor/metricsgenerationprocessor/

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor => ./processor/probabilisticsamplerprocessor/

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatorateprocessor => ./processor/deltatorateprocessor/

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/routingprocessor => ./processor/routingprocessor/

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor => ./processor/tailsamplingprocessor

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanmetricsprocessor => ./processor/spanmetricsprocessor/

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanprocessor => ./processor/spanprocessor/

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor => ./processor/cumulativetodeltaprocessor/

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudexporter => ./exporter/googlecloudexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stackdriverexporter => ./exporter/stackdriverexporter

// see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/4433
exclude github.com/StackExchange/wmi v1.2.0

retract v0.37.0 // Contains dependencies on v0.36.0 components, which should have been updated to v0.37.0.
