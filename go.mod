module github.com/open-telemetry/opentelemetry-collector-contrib

go 1.18

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudlogserviceexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsprometheusremotewriteexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/carbonexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dynatraceexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/f5cloudexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudpubsubexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlemanagedprometheusexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/humioexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/influxdbexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/jaegerexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/jaegerthrifthttpexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logzioexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mezmoexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opencensusexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/parquetexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/pulsarexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sentryexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/skywalkingexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tanzuobservabilityexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tencentcloudlogserviceexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/zipkinexporter v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/asapauthextension v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/awsproxy v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/bearertokenauthextension v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/fluentbitextension v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/headerssetter v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/httpforwarder v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecstaskobserver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/hostobserver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/oidcauthextension v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/sigv4authextension v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatorateprocessor v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricsgenerationprocessor v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/routingprocessor v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanmetricsprocessor v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/popularityfilterprocessor v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanprocessor v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/activedirectorydsreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachereceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/bigipreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/chronyreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudfoundryreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/couchdbreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dotnetdiagnosticsreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/expvarreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/flinkmetricsreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fluentforwardreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/influxdbreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/journaldreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8seventsreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/memcachedreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nginxreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nsxtreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/opencensusreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpjsonfilereceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexecreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/riakreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/saphanareceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/skywalkingreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/syslogreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcplogreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/udplogreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/wavefrontreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowseventlogreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsperfcountersreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver v0.57.2
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zookeeperreceiver v0.57.2
	github.com/prometheus/prometheus v0.37.0
	github.com/stretchr/testify v1.8.0
	go.opentelemetry.io/collector v0.57.2
	go.uber.org/multierr v1.8.0
	golang.org/x/sys v0.0.0-20220627191245-f75cf1eec38b
)

require (
	bitbucket.org/atlassian/go-asap/v2 v2.6.0 // indirect
	cloud.google.com/go v0.102.1 // indirect
	cloud.google.com/go/compute v1.7.0 // indirect
	cloud.google.com/go/iam v0.3.0 // indirect
	cloud.google.com/go/logging v1.4.2 // indirect
	cloud.google.com/go/monitoring v1.5.0 // indirect
	cloud.google.com/go/pubsub v1.24.0 // indirect
	cloud.google.com/go/spanner v1.36.0 // indirect
	cloud.google.com/go/trace v1.2.0 // indirect
	code.cloudfoundry.org/clock v1.0.0 // indirect
	code.cloudfoundry.org/go-diodes v0.0.0-20211115184647-b584dd5df32c // indirect
	code.cloudfoundry.org/go-loggregator v7.4.0+incompatible // indirect
	code.cloudfoundry.org/rfc5424 v0.0.0-20201103192249-000122071b78 // indirect
	contrib.go.opencensus.io/exporter/prometheus v0.4.1 // indirect
	contrib.go.opencensus.io/exporter/stackdriver v0.13.13 // indirect
	github.com/99designs/keyring v1.1.6 // indirect
	github.com/AthenZ/athenz v1.10.39 // indirect
	github.com/Azure/azure-pipeline-go v0.2.3 // indirect
	github.com/Azure/azure-sdk-for-go v65.0.0+incompatible // indirect
	github.com/Azure/azure-storage-blob-go v0.14.0 // indirect
	github.com/Azure/go-amqp v0.17.5 // indirect
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest v0.11.27 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.20 // indirect
	github.com/Azure/go-autorest/autorest/date v0.3.0 // indirect
	github.com/Azure/go-autorest/autorest/to v0.4.0 // indirect
	github.com/Azure/go-autorest/autorest/validation v0.3.1 // indirect
	github.com/Azure/go-autorest/logger v0.2.1 // indirect
	github.com/Azure/go-autorest/tracing v0.6.0 // indirect
	github.com/ClickHouse/clickhouse-go/v2 v2.2.0 // indirect
	github.com/DataDog/agent-payload/v5 v5.0.27 // indirect
	github.com/DataDog/datadog-agent/pkg/obfuscate v0.38.0-rc.3.0.20220804102556-2fec6abdb5f7 // indirect
	github.com/DataDog/datadog-agent/pkg/otlp/model v0.38.0-rc.3.0.20220804102556-2fec6abdb5f7 // indirect
	github.com/DataDog/datadog-agent/pkg/quantile v0.38.0-rc.3.0.20220804102556-2fec6abdb5f7 // indirect
	github.com/DataDog/datadog-agent/pkg/remoteconfig/state v0.38.0-rc.3.0.20220804102556-2fec6abdb5f7 // indirect
	github.com/DataDog/datadog-agent/pkg/trace v0.38.0-rc.3.0.20220804102556-2fec6abdb5f7 // indirect
	github.com/DataDog/datadog-go/v5 v5.1.1 // indirect
	github.com/DataDog/gohai v0.0.0-20220718130825-1776f9beb9cc // indirect
	github.com/DataDog/sketches-go v1.4.1 // indirect
	github.com/DataDog/zstd v1.5.2 // indirect
	github.com/GehirnInc/crypt v0.0.0-20200316065508-bb7000b8a962 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v0.32.5 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector v0.32.5 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/googlemanagedprometheus v0.32.5 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v1.8.5 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.32.5 // indirect
	github.com/Microsoft/go-winio v0.5.2 // indirect
	github.com/ReneKroon/ttlcache/v2 v2.11.0 // indirect
	github.com/SAP/go-hdb v0.106.1 // indirect
	github.com/SermoDigital/jose v0.9.2-0.20161205224733-f6df55f235c2 // indirect
	github.com/Shopify/sarama v1.35.0 // indirect
	github.com/Showmax/go-fqdn v1.0.0 // indirect
	github.com/VividCortex/gohistogram v1.0.0 // indirect
	github.com/aerospike/aerospike-client-go/v5 v5.8.0 // indirect
	github.com/alecthomas/participle/v2 v2.0.0-alpha9 // indirect
	github.com/alecthomas/units v0.0.0-20211218093645-b94a6e3cc137 // indirect
	github.com/aliyun/aliyun-log-go-sdk v0.1.37 // indirect
	github.com/antonmedv/expr v1.9.0 // indirect
	github.com/apache/arrow/go/arrow v0.0.0-20211112161151-bc219186db40 // indirect
	github.com/apache/pulsar-client-go v0.8.1 // indirect
	github.com/apache/pulsar-client-go/oauth2 v0.0.0-20220120090717-25e59572242e // indirect
	github.com/apache/thrift v0.16.0 // indirect
	github.com/ardielle/ardielle-go v1.5.2 // indirect
	github.com/armon/go-metrics v0.3.10 // indirect
	github.com/aws/aws-sdk-go v1.44.72 // indirect
	github.com/aws/aws-sdk-go-v2 v1.11.0 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.0.0 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.10.1 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.6.1 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.8.0 // indirect
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.7.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.1.0 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.0.0 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.3.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.5.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.5.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.9.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/s3 v1.19.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.6.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.10.0 // indirect
	github.com/aws/smithy-go v1.9.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver v3.5.1+incompatible // indirect
	github.com/bmatcuk/doublestar/v3 v3.0.0 // indirect
	github.com/caio/go-tdigest v3.1.0+incompatible // indirect
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/cenkalti/backoff/v4 v4.1.3 // indirect
	github.com/census-instrumentation/opencensus-proto v0.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/checkpoint-restore/go-criu/v5 v5.3.0 // indirect
	github.com/cihub/seelog v0.0.0-20151216151435-d2c6e5aa9fbf // indirect
	github.com/cilium/ebpf v0.8.1 // indirect
	github.com/cloudfoundry-incubator/uaago v0.0.0-20190307164349-8136b7bbe76e // indirect
	github.com/cncf/udpa/go v0.0.0-20210930031921-04548b0d99d4 // indirect
	github.com/cncf/xds/go v0.0.0-20220314180256-7f1daf1720fc // indirect
	github.com/containerd/console v1.0.3 // indirect
	github.com/containerd/containerd v1.6.1 // indirect
	github.com/containerd/ttrpc v1.1.0 // indirect
	github.com/coralogix/opentelemetry-cx-protobuf-api/coralogixpb v0.0.0-20211201100428-d2a5d0ecf53e // indirect
	github.com/coreos/go-oidc v2.2.1+incompatible // indirect
	github.com/coreos/go-systemd/v22 v22.3.2 // indirect
	github.com/cyphar/filepath-securejoin v0.2.3 // indirect
	github.com/danieljoos/wincred v1.0.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/denisenkom/go-mssqldb v0.12.2 // indirect
	github.com/dennwc/varint v1.0.0 // indirect
	github.com/dgraph-io/ristretto v0.1.0 // indirect
	github.com/dgryski/go-farm v0.0.0-20200201041132-a6ae2369ad13 // indirect
	github.com/digitalocean/godo v1.81.0 // indirect
	github.com/docker/distribution v2.8.1+incompatible // indirect
	github.com/docker/docker v20.10.17+incompatible // indirect
	github.com/docker/go-connections v0.4.1-0.20210727194412-58542c764a11 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/dvsekhvalnov/jose2go v0.0.0-20200901110807-248326c1351b // indirect
	github.com/dynatrace-oss/dynatrace-metric-utils-go v0.5.0 // indirect
	github.com/eapache/go-resiliency v1.3.0 // indirect
	github.com/eapache/go-xerial-snappy v0.0.0-20180814174437-776d5712da21 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/elastic/go-elasticsearch/v7 v7.17.1 // indirect
	github.com/elastic/go-structform v0.0.10 // indirect
	github.com/emicklei/go-restful v2.9.5+incompatible // indirect
	github.com/envoyproxy/go-control-plane v0.10.3 // indirect
	github.com/envoyproxy/protoc-gen-validate v0.6.7 // indirect
	github.com/euank/go-kmsg-parser v2.0.0+incompatible // indirect
	github.com/facebook/time v0.0.0-20220713225404-f7a0d7702d50 // indirect
	github.com/facebookgo/clock v0.0.0-20150410010913-600d898af40a // indirect
	github.com/facebookgo/limitgroup v0.0.0-20150612190941-6abd8d71ec01 // indirect
	github.com/facebookgo/muster v0.0.0-20150708232844-fd3d7953fd52 // indirect
	github.com/fatih/color v1.13.0 // indirect
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/form3tech-oss/jwt-go v3.2.5+incompatible // indirect
	github.com/fsnotify/fsnotify v1.5.4 // indirect
	github.com/gabriel-vasile/mimetype v1.4.0 // indirect
	github.com/getsentry/sentry-go v0.13.0 // indirect
	github.com/go-kit/kit v0.12.0 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-logfmt/logfmt v0.5.1 // indirect
	github.com/go-logr/logr v1.2.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/jsonreference v0.20.0 // indirect
	github.com/go-openapi/swag v0.21.1 // indirect
	github.com/go-redis/redis/v7 v7.4.1 // indirect
	github.com/go-resty/resty/v2 v2.1.1-0.20191201195748-d7b97669fe48 // indirect
	github.com/go-sql-driver/mysql v1.6.0 // indirect
	github.com/go-stack/stack v1.8.1 // indirect
	github.com/go-zookeeper/zk v1.0.2 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/godbus/dbus v0.0.0-20190726142602-4481cbc300e2 // indirect
	github.com/godbus/dbus/v5 v5.0.6 // indirect
	github.com/gofrs/uuid v4.2.0+incompatible // indirect
	github.com/gogo/googleapis v1.4.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt v3.2.2+incompatible // indirect
	github.com/golang-jwt/jwt/v4 v4.2.0 // indirect
	github.com/golang-sql/civil v0.0.0-20190719163853-cb61b32ac6fe // indirect
	github.com/golang-sql/sqlexp v0.1.0 // indirect
	github.com/golang/glog v1.0.0 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/cadvisor v0.45.0 // indirect
	github.com/google/flatbuffers v2.0.0+incompatible // indirect
	github.com/google/gnostic v0.5.7-v3refs // indirect
	github.com/google/go-cmp v0.5.8 // indirect
	github.com/google/go-querystring v1.1.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.1.0 // indirect
	github.com/googleapis/gax-go/v2 v2.4.0 // indirect
	github.com/gophercloud/gophercloud v0.25.0 // indirect
	github.com/gorilla/handlers v1.5.1 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/grafana/regexp v0.0.0-20220304095617-2e8d9baf4ac2 // indirect
	github.com/grobie/gomemcache v0.0.0-20180201122607-1f779c573665 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware v1.3.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.16.0 // indirect
	github.com/gsterjov/go-libsecret v0.0.0-20161001094733-a6f4afe4910c // indirect
	github.com/hashicorp/consul/api v1.13.1 // indirect
	github.com/hashicorp/cronexpr v1.1.1 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-hclog v1.2.2 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.1 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-uuid v1.0.2 // indirect
	github.com/hashicorp/go-version v1.6.0 // indirect
	github.com/hashicorp/golang-lru v0.5.4 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/hashicorp/memberlist v0.3.1 // indirect
	github.com/hashicorp/nomad/api v0.0.0-20220718181929-9457a13c7ccf // indirect
	github.com/hashicorp/serf v0.9.7 // indirect
	github.com/hetznercloud/hcloud-go v1.35.0 // indirect
	github.com/honeycombio/libhoney-go v1.16.0 // indirect
	github.com/iancoleman/strcase v0.2.0 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/influxdata/go-syslog/v3 v3.0.1-0.20210608084020-ac565dc76ba6 // indirect
	github.com/influxdata/influxdb-observability/common v0.2.26 // indirect
	github.com/influxdata/influxdb-observability/influx2otel v0.2.26 // indirect
	github.com/influxdata/influxdb-observability/otel2influx v0.2.26 // indirect
	github.com/influxdata/line-protocol/v2 v2.2.1 // indirect
	github.com/ionos-cloud/sdk-go/v6 v6.1.0 // indirect
	github.com/jackc/chunkreader/v2 v2.0.1 // indirect
	github.com/jackc/pgconn v1.13.0 // indirect
	github.com/jackc/pgio v1.0.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgproto3/v2 v2.3.1 // indirect
	github.com/jackc/pgservicefile v0.0.0-20200714003250-2b9c44734f2b // indirect
	github.com/jackc/pgtype v1.12.0 // indirect
	github.com/jackc/pgx/v4 v4.17.0 // indirect
	github.com/jaegertracing/jaeger v1.37.0 // indirect
	github.com/jcmturner/aescts/v2 v2.0.0 // indirect
	github.com/jcmturner/dnsutils/v2 v2.0.0 // indirect
	github.com/jcmturner/gofork v1.0.0 // indirect
	github.com/jcmturner/gokrb5/v8 v8.4.2 // indirect
	github.com/jcmturner/rpc/v2 v2.0.3 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/karrick/godirwalk v1.16.1 // indirect
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51 // indirect
	github.com/keybase/go-keychain v0.0.0-20190712205309-48d3d31d256d // indirect
	github.com/klauspost/compress v1.15.9 // indirect
	github.com/knadh/koanf v1.4.2 // indirect
	github.com/kolo/xmlrpc v0.0.0-20201022064351-38db28db192b // indirect
	github.com/leoluk/perflib_exporter v0.1.0 // indirect
	github.com/lib/pq v1.10.6 // indirect
	github.com/linkedin/goavro/v2 v2.9.8 // indirect
	github.com/linode/linodego v1.8.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/magiconair/properties v1.8.6 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.12 // indirect
	github.com/mattn/go-ieproxy v0.0.1 // indirect
	github.com/mattn/go-isatty v0.0.14 // indirect
	github.com/mattn/go-sqlite3 v2.0.3+incompatible // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/microsoft/ApplicationInsights-Go v0.4.4 // indirect
	github.com/miekg/dns v1.1.50 // indirect
	github.com/mindprince/gonvml v0.0.0-20190828220739-9ebdce4bb989 // indirect
	github.com/mistifyio/go-zfs v2.1.2-0.20190413222219-f784269be439+incompatible // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/hashstructure v1.1.0 // indirect
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/sys/mountinfo v0.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/mongodb-forks/digest v1.0.4 // indirect
	github.com/montanaflynn/stats v0.0.0-20171201202039-1bf9dbcd8cbe // indirect
	github.com/mostynb/go-grpc-compression v1.1.17 // indirect
	github.com/mrunalp/fileutils v0.5.0 // indirect
	github.com/mtibben/percent v0.2.1 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f // indirect
	github.com/nginxinc/nginx-prometheus-exporter v0.8.1-0.20201110005315-f5a5f8086c19 // indirect
	github.com/observiq/ctimefmt v1.0.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer v0.57.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil v0.57.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight v0.57.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/cwlogs v0.57.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil v0.57.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s v0.57.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics v0.57.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy v0.57.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray v0.57.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker v0.57.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig v0.57.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/kubelet v0.57.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders v0.57.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent v0.57.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk v0.57.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr v0.57.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal v0.57.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata v0.57.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry v0.57.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage v0.57.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger v0.57.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus v0.57.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus v0.57.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite v0.57.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/signalfx v0.57.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin v0.57.2 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters v0.57.2 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.2 // indirect
	github.com/opencontainers/runc v1.1.3 // indirect
	github.com/opencontainers/runtime-spec v1.0.3-0.20210326190908-1c3f411f0417 // indirect
	github.com/opencontainers/selinux v1.10.0 // indirect
	github.com/openlyinc/pointy v1.1.2 // indirect
	github.com/openshift/api v0.0.0-20210521075222-e273a339932a // indirect
	github.com/openshift/client-go v0.0.0-20210521082421-73d9475a9142 // indirect
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/openzipkin/zipkin-go v0.4.0 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/paulmach/orb v0.7.1 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/pelletier/go-toml/v2 v2.0.1 // indirect
	github.com/philhofer/fwd v1.1.1 // indirect
	github.com/pierrec/lz4 v2.6.1+incompatible // indirect
	github.com/pierrec/lz4/v4 v4.1.15 // indirect
	github.com/pkg/browser v0.0.0-20210911075715-681adbf594b8 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/pquerna/cachecontrol v0.1.0 // indirect
	github.com/prometheus/client_golang v1.13.0 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.37.0 // indirect
	github.com/prometheus/common/sigv4 v0.1.0 // indirect
	github.com/prometheus/procfs v0.8.0 // indirect
	github.com/prometheus/statsd_exporter v0.21.0 // indirect
	github.com/rcrowley/go-metrics v0.0.0-20201227073835-cf1acfcdf475 // indirect
	github.com/rs/cors v1.8.2 // indirect
	github.com/santhosh-tekuri/jsonschema v1.2.4 // indirect
	github.com/scaleway/scaleway-sdk-go v1.0.0-beta.9 // indirect
	github.com/seccomp/libseccomp-golang v0.9.2-0.20220502022130-f33da4d89646 // indirect
	github.com/secure-systems-lab/go-securesystemslib v0.3.1 // indirect
	github.com/shirou/gopsutil/v3 v3.22.7 // indirect
	github.com/shopspring/decimal v1.3.1 // indirect
	github.com/signalfx/com_signalfx_metrics_protobuf v0.0.3 // indirect
	github.com/signalfx/gohistogram v0.0.0-20160107210732-1ccfd2ff5083 // indirect
	github.com/signalfx/golib/v3 v3.3.13 // indirect
	github.com/signalfx/sapm-proto v0.11.0 // indirect
	github.com/signalfx/signalfx-agent/pkg/apm v0.0.0-20201202163743-65b4fa925fc8 // indirect
	github.com/sijms/go-ora/v2 v2.4.26 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/snowflakedb/gosnowflake v1.6.10 // indirect
	github.com/soheilhy/cmux v0.1.5 // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	github.com/spf13/afero v1.8.2 // indirect
	github.com/spf13/cast v1.5.0 // indirect
	github.com/spf13/cobra v1.5.0 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/spf13/viper v1.12.0 // indirect
	github.com/stretchr/objx v0.4.0 // indirect
	github.com/subosito/gotenv v1.3.0 // indirect
	github.com/syndtr/gocapability v0.0.0-20200815063812-42c35b437635 // indirect
	github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common v1.0.467 // indirect
	github.com/tg123/go-htpasswd v1.2.0 // indirect
	github.com/theupdateframework/go-tuf v0.3.0 // indirect
	github.com/tidwall/gjson v1.10.2 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	github.com/tidwall/tinylru v1.1.0 // indirect
	github.com/tidwall/wal v1.1.7 // indirect
	github.com/tilinna/clock v1.1.0 // indirect
	github.com/tinylib/msgp v1.1.6 // indirect
	github.com/tklauser/go-sysconf v0.3.10 // indirect
	github.com/tklauser/numcpus v0.4.0 // indirect
	github.com/uber/jaeger-client-go v2.30.0+incompatible // indirect
	github.com/uber/jaeger-lib v2.4.1+incompatible // indirect
	github.com/vincent-petithory/dataurl v1.0.0 // indirect
	github.com/vishvananda/netlink v1.1.1-0.20210330154013-f5de75959ad5 // indirect
	github.com/vishvananda/netns v0.0.0-20210104183010-2eb08e3e575f // indirect
	github.com/vmihailenco/msgpack/v5 v5.3.5 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	github.com/vmware/go-vmware-nsxt v0.0.0-20220328155605-f49a14c1ef5f // indirect
	github.com/vmware/govmomi v0.28.0 // indirect
	github.com/vultr/govultr/v2 v2.17.2 // indirect
	github.com/wavefronthq/wavefront-sdk-go v0.9.11 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.1.1 // indirect
	github.com/xdg-go/stringprep v1.0.3 // indirect
	github.com/youmark/pkcs8 v0.0.0-20181117223130-1be2e3e5546d // indirect
	github.com/yuin/gopher-lua v0.0.0-20200816102855-ee81675732da // indirect
	github.com/yusufpapurcu/wmi v1.2.2 // indirect
	go.elastic.co/apm v1.15.0 // indirect
	go.elastic.co/fastjson v1.1.0 // indirect
	go.etcd.io/bbolt v1.3.6 // indirect
	go.mongodb.org/atlas v0.16.0 // indirect
	go.mongodb.org/mongo-driver v1.10.1 // indirect
	go.opencensus.io v0.23.0 // indirect
	go.opentelemetry.io/collector/pdata v0.57.2 // indirect
	go.opentelemetry.io/collector/semconv v0.57.2 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.34.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.34.0 // indirect
	go.opentelemetry.io/contrib/zpages v0.33.0 // indirect
	go.opentelemetry.io/otel v1.9.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.31.0 // indirect
	go.opentelemetry.io/otel/metric v0.31.0 // indirect
	go.opentelemetry.io/otel/sdk v1.9.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v0.31.0 // indirect
	go.opentelemetry.io/otel/trace v1.9.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/goleak v1.1.12 // indirect
	go.uber.org/zap v1.22.0 // indirect
	golang.org/x/crypto v0.0.0-20220722155217-630584e8d5aa // indirect
	golang.org/x/mod v0.6.0-dev.0.20220419223038-86c51ed26bb4 // indirect
	golang.org/x/net v0.0.0-20220708220712-1185a9018129 // indirect
	golang.org/x/oauth2 v0.0.0-20220628200809-02e64fa58f26 // indirect
	golang.org/x/sync v0.0.0-20220601150217-0de741cfad7f // indirect
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211 // indirect
	golang.org/x/text v0.3.7 // indirect
	golang.org/x/time v0.0.0-20220609170525-579cf78fd858 // indirect
	golang.org/x/tools v0.1.11 // indirect
	golang.org/x/xerrors v0.0.0-20220609144429-65e65417b02f // indirect
	gonum.org/v1/gonum v0.11.0 // indirect
	google.golang.org/api v0.91.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20220706185917-7780775163c4 // indirect
	google.golang.org/grpc v1.48.0 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/alexcesaro/statsd.v2 v2.0.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.66.4 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
	gopkg.in/square/go-jose.v2 v2.5.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	gopkg.in/zorkian/go-datadog-api.v2 v2.30.0 // indirect
	k8s.io/api v0.24.3 // indirect
	k8s.io/apimachinery v0.24.3 // indirect
	k8s.io/client-go v0.24.3 // indirect
	k8s.io/klog v1.0.0 // indirect
	k8s.io/klog/v2 v2.70.0 // indirect
	k8s.io/kube-openapi v0.0.0-20220328201542-3ee0da9b0b42 // indirect
	k8s.io/kubelet v0.24.0 // indirect
	k8s.io/utils v0.0.0-20220210201930-3a6ce19ff2f9 // indirect
	sigs.k8s.io/json v0.0.0-20211208200746-9f7c6b3444d2 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.1 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
	skywalking.apache.org/repo/goapi v0.0.0-20220121092418-9c455d0dda3f // indirect
)

// Replace references to modules that are in this repository with their relateive paths
// so that we always build with current (latest) version of the source code.

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil => ./internal/aws/awsutil

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight => ./internal/aws/containerinsight

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/cwlogs => ./internal/aws/cwlogs

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil => ./internal/aws/ecsutil

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s => ./internal/aws/k8s

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics => ./internal/aws/metrics

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy => ./internal/aws/proxy

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray => ./internal/aws/xray

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ./internal/common

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/containertest => ./internal/containertest

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ./internal/coreinternal

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker => ./internal/docker

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig => ./internal/k8sconfig

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/kubelet => ./internal/kubelet

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders => ./internal/metadataproviders

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/scrapertest => ./internal/scrapertest

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent => ./internal/sharedcomponent

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk => ./internal/splunk

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudlogserviceexporter => ./exporter/alibabacloudlogserviceexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter => ./exporter/awscloudwatchlogsexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter => ./exporter/awsemfexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter => ./exporter/awskinesisexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsprometheusremotewriteexporter => ./exporter/awsprometheusremotewriteexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter => ./exporter/awsxrayexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter => ./exporter/azuremonitorexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/carbonexporter => ./exporter/carbonexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter => ./exporter/clickhouseexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter => ./exporter/coralogixexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter => ./exporter/datadogexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dynatraceexporter => ./exporter/dynatraceexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/f5cloudexporter => ./exporter/f5cloudexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter => ./exporter/fileexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudexporter => ./exporter/googlecloudexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlemanagedprometheusexporter => ./exporter/googlemanagedprometheusexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudpubsubexporter => ./exporter/googlecloudpubsubexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombexporter => ./exporter/honeycombexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/humioexporter => ./exporter/humioexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/influxdbexporter => ./exporter/influxdbexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/jaegerexporter => ./exporter/jaegerexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/jaegerthrifthttpexporter => ./exporter/jaegerthrifthttpexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter => ./exporter/kafkaexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter => ./exporter/loadbalancingexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logzioexporter => ./exporter/logzioexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lokiexporter => ./exporter/lokiexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mezmoexporter => ./exporter/mezmoexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opencensusexporter => ./exporter/opencensusexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/parquetexporter => ./exporter/parquetexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter => ./exporter/prometheusexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter => ./exporter/prometheusremotewriteexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/pulsarexporter => ./exporter/pulsarexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter => ./exporter/sapmexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sentryexporter => ./exporter/sentryexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter => ./exporter/signalfxexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/skywalkingexporter => ./exporter/skywalkingexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter => ./exporter/splunkhecexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter => ./exporter/sumologicexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tanzuobservabilityexporter => ./exporter/tanzuobservabilityexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tencentcloudlogserviceexporter => ./exporter/tencentcloudlogserviceexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticexporter => ./exporter/elasticexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter => ./exporter/elasticsearchexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/zipkinexporter => ./exporter/zipkinexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/asapauthextension => ./extension/asapauthextension

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/awsproxy => ./extension/awsproxy

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension => ./extension/basicauthextension

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/bearertokenauthextension => ./extension/bearertokenauthextension

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/fluentbitextension => ./extension/fluentbitextension

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/headerssetter => ./extension/headerssetter

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension => ./extension/healthcheckextension

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/httpforwarder => ./extension/httpforwarder

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling => ./extension/jaegerremotesampling

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension => ./extension/oauth2clientauthextension

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer => ./extension/observer

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecstaskobserver => ./extension/observer/ecstaskobserver

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/hostobserver => ./extension/observer/hostobserver

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver => ./extension/observer/k8sobserver

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/oidcauthextension => ./extension/oidcauthextension

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension => ./extension/pprofextension

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/sigv4authextension => ./extension/sigv4authextension

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage => ./extension/storage

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr => ./pkg/batchperresourceattr

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal => ./pkg/batchpersignal

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters => ./pkg/winperfcounters

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata => ./pkg/experimentalmetricmetadata

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry => ./pkg/resourcetotelemetry

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza => ./pkg/stanza

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/telemetryquerylanguage => ./pkg/telemetryquerylanguage

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger => ./pkg/translator/jaeger

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus => ./pkg/translator/opencensus

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus => ./pkg/translator/prometheus

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite => ./pkg/translator/prometheusremotewrite

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/signalfx => ./pkg/translator/signalfx

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin => ./pkg/translator/zipkin

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor => ./processor/attributesprocessor

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor => ./processor/cumulativetodeltaprocessor/

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatorateprocessor => ./processor/deltatorateprocessor/

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor => ./processor/filterprocessor

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor => ./processor/groupbyattrsprocessor

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor => ./processor/groupbytraceprocessor

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor => ./processor/k8sattributesprocessor/

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricsgenerationprocessor => ./processor/metricsgenerationprocessor/

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor => ./processor/metricstransformprocessor/

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor => ./processor/probabilisticsamplerprocessor/

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor => ./processor/resourcedetectionprocessor/

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor => ./processor/resourceprocessor/

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/routingprocessor => ./processor/routingprocessor/

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanmetricsprocessor => ./processor/spanmetricsprocessor/

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/popularityfilterprocessor => ./processor/popularityfilterprocessor/

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanprocessor => ./processor/spanprocessor/

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor => ./processor/tailsamplingprocessor

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor => ./processor/transformprocessor

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor => ./processor/schemaprocessor

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/activedirectorydsreceiver => ./receiver/activedirectorydsreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver => ./receiver/aerospikereceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachereceiver => ./receiver/apachereceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver => ./receiver/awscontainerinsightreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver => ./receiver/awsecscontainermetricsreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver => ./receiver/awsfirehosereceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver => ./receiver/awsxrayreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver => ./receiver/azureeventhubreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/bigipreceiver => ./receiver/bigipreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver => ./receiver/carbonreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/chronyreceiver => ./receiver/chronyreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudfoundryreceiver => ./receiver/cloudfoundryreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver => ./receiver/collectdreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/couchdbreceiver => ./receiver/couchdbreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/flinkmetricsreceiver => ./receiver/flinkmetricsreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver => ./receiver/dockerstatsreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dotnetdiagnosticsreceiver => ./receiver/dotnetdiagnosticsreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver => ./receiver/elasticsearchreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/expvarreceiver => ./receiver/expvarreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver => ./receiver/filelogreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver => ./receiver/googlecloudspannerreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver => ./receiver/googlecloudpubsubreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver => ./receiver/hostmetricsreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/influxdbreceiver => ./receiver/influxdbreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver => ./receiver/iisreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver => ./receiver/jaegerreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver => ./receiver/jmxreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/journaldreceiver => ./receiver/journaldreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver => ./receiver/k8sclusterreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8seventsreceiver => ./receiver/k8seventsreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fluentforwardreceiver => ./receiver/fluentforwardreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver => ./receiver/kafkametricsreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver => ./receiver/kafkareceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver => ./receiver/kubeletstatsreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/memcachedreceiver => ./receiver/memcachedreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver => ./receiver/mongodbreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver => ./receiver/mongodbatlasreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver => ./receiver/mysqlreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nginxreceiver => ./receiver/nginxreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nsxtreceiver => ./receiver/nsxtreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/opencensusreceiver => ./receiver/opencensusreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpjsonfilereceiver => ./receiver/otlpjsonfilereceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver => ./receiver/podmanreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver => ./receiver/postgresqlreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexecreceiver => ./receiver/prometheusexecreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver => ./receiver/prometheusreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver => ./receiver/rabbitmqreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator => ./receiver/receivercreator

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver => ./receiver/redisreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/riakreceiver => ./receiver/riakreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/saphanareceiver => ./receiver/saphanareceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver => ./receiver/sapmreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver => ./receiver/signalfxreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver => ./receiver/simpleprometheusreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/skywalkingreceiver => ./receiver/skywalkingreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver => ./receiver/solacereceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver => ./receiver/splunkhecreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver => ./receiver/sqlqueryreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver => ./receiver/sqlserverreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver => ./receiver/statsdreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/syslogreceiver => ./receiver/syslogreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcplogreceiver => ./receiver/tcplogreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/udplogreceiver => ./receiver/udplogreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver => ./receiver/vcenterreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/wavefrontreceiver => ./receiver/wavefrontreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowseventlogreceiver => ./receiver/windowseventlogreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsperfcountersreceiver => ./receiver/windowsperfcountersreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver => ./receiver/zipkinreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zookeeperreceiver => ./receiver/zookeeperreceiver

// see https://github.com/google/gnostic/issues/262
replace github.com/googleapis/gnostic v0.5.6 => github.com/googleapis/gnostic v0.5.5

// see https://github.com/go-chi/chi/issues/713
// see https://github.com/DataDog/dd-trace-go/issues/1220
replace github.com/go-chi/chi/v4 => github.com/go-chi/chi v4.0.0+incompatible

// see https://github.com/open-telemetry/opentelemetry-collector-contrib/pull/12322#issuecomment-1185029670
replace github.com/docker/go-connections v0.4.1-0.20210727194412-58542c764a11 => github.com/docker/go-connections v0.4.0

// see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/4433
exclude github.com/StackExchange/wmi v1.2.0

retract v0.37.0 // Contains dependencies on v0.36.0 components, which should have been updated to v0.37.0.

// see https://github.com/distribution/distribution/issues/3590
exclude github.com/docker/distribution v2.8.0+incompatible

// see https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/12934
replace github.com/outcaste-io/ristretto => github.com/outcaste-io/ristretto v0.1.1-0.20220420003845-bd658a460d49
