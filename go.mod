module github.com/open-telemetry/opentelemetry-collector-contrib

// NOTE:
// This go.mod is NOT used to build any official binary.
// To see the builder manifests used for official binaries,
// check https://github.com/open-telemetry/opentelemetry-collector-releases
//
// For the OpenTelemetry Collector Contrib distribution specifically, see
// https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib

go 1.25.0

retract (
	v0.76.2
	v0.76.1
	v0.65.0
	v0.37.0 // Contains dependencies on v0.36.0 components, which should have been updated to v0.37.0.
)

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/confmap/provider/aesprovider v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/confmap/provider/googlesecretmanagerprovider v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/confmap/provider/s3provider v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/confmap/provider/secretsmanagerprovider v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/connector/countconnector v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/connector/datadogconnector v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/connector/exceptionsconnector v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/connector/failoverconnector v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/connector/grafanacloudconnector v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/connector/otlpjsonconnector v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/connector/roundrobinconnector v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/connector/routingconnector v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/connector/servicegraphconnector v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/connector/sumconnector v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alertmanagerexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudlogserviceexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awscloudwatchlogsexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awss3exporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureblobexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuredataexplorerexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/bmchelixexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/cassandraexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/coralogixexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datasetexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dorisexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/faroexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/fileexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudpubsubexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudstorageexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlemanagedprometheusexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombmarkerexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/influxdbexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logicmonitorexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logzioexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/mezmoexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/otelarrowexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/prometheusremotewriteexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/pulsarexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/rabbitmqexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter v0.147.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sentryexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stefexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/syslogexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tencentcloudlogserviceexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/tinybirdexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/zipkinexporter v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/ackextension v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/asapauthextension v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/awsproxy v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/azureauthextension v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/basicauthextension v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/bearertokenauthextension v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/cgroupruntimeextension v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/datadogextension v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/avrologencodingextension v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awscloudwatchmetricstreamsencodingextension v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/awslogsencodingextension v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/azureencodingextension v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/googlecloudlogentryencodingextension v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/jaegerencodingextension v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/jsonlogencodingextension v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/otlpencodingextension v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/skywalkingencodingextension v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/textencodingextension v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/zipkinencodingextension v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/googleclientauthextension v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/headerssetterextension v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckextension v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/healthcheckv2extension v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/httpforwarderextension v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/jaegerremotesampling v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/k8sleaderelector v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/oauth2clientauthextension v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/cfgardenobserver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/dockerobserver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/ecsobserver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/hostobserver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/kafkatopicsobserver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/oidcauthextension v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampextension v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/pprofextension v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/remotetapextension v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/sigv4authextension v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/solarwindsapmsettingsextension v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/dbstorage v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/filestorage v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/storage/redisstorageextension v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/sumologicextension v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/coralogixprocessor v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/datadogsemanticsprocessor v0.147.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatocumulativeprocessor v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/deltatorateprocessor v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/geoipprocessor v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbyattrsprocessor v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/intervalprocessor v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/isolationforestprocessor v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sattributesprocessor v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/logdedupprocessor v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricsgenerationprocessor v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/remotetapprocessor v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourceprocessor v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/schemaprocessor v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanprocessor v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/sumologicprocessor v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/transformprocessor v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/unrollprocessor v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/activedirectorydsreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/aerospikereceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachereceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachesparkreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureblobreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azureeventhubreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/azuremonitorreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/bigipreceiver v0.146.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/chronyreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ciscoosreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudflarereceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/cloudfoundryreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/couchdbreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/datadogreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/elasticsearchreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/envoyalsreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/expvarreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/faroreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filelogreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/filestatsreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/flinkmetricsreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/fluentforwardreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/gitlabreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudmonitoringreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudspannerreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/haproxyreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/httpcheckreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/huaweicloudcesreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/iisreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/influxdbreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver v0.156.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/journaldreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8seventsreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sobjectsreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkametricsreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/lokireceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/memcachedreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbatlasreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mysqlreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/namedpipereceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/netflowreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nginxreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nsxtreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/ntpreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/oracledbreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otelarrowreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/otlpjsonfilereceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusremotewritereceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/pulsarreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefareceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/purefbreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/rabbitmqreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redfishreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/riakreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/saphanareceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/skywalkingreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snmpreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/snowflakereceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/solacereceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlqueryreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sqlserverreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sshcheckreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/stefreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/syslogreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/systemdreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcplogreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/udplogreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/vcenterreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/wavefrontreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/webhookeventreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowseventlogreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsperfcountersreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver v0.157.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zookeeperreceiver v0.157.0
	go.opentelemetry.io/collector/component v1.63.0
	go.opentelemetry.io/collector/confmap v1.63.0
	go.opentelemetry.io/collector/confmap/provider/envprovider v1.63.0
	go.opentelemetry.io/collector/confmap/provider/fileprovider v1.63.0
	go.opentelemetry.io/collector/confmap/provider/httpprovider v1.63.0
	go.opentelemetry.io/collector/confmap/provider/httpsprovider v1.63.0
	go.opentelemetry.io/collector/confmap/provider/yamlprovider v1.63.0
	go.opentelemetry.io/collector/connector v0.157.0
	go.opentelemetry.io/collector/connector/forwardconnector v0.157.0
	go.opentelemetry.io/collector/exporter v1.63.0
	go.opentelemetry.io/collector/exporter/debugexporter v0.157.0
	go.opentelemetry.io/collector/exporter/nopexporter v0.157.0
	go.opentelemetry.io/collector/exporter/otlpexporter v0.157.0
	go.opentelemetry.io/collector/exporter/otlphttpexporter v0.157.0
	go.opentelemetry.io/collector/extension v1.63.0
	go.opentelemetry.io/collector/extension/zpagesextension v0.157.0
	go.opentelemetry.io/collector/otelcol v0.157.0
	go.opentelemetry.io/collector/processor v1.63.0
	go.opentelemetry.io/collector/processor/batchprocessor v0.157.0
	go.opentelemetry.io/collector/processor/memorylimiterprocessor v0.157.0
	go.opentelemetry.io/collector/receiver v1.63.0
	go.opentelemetry.io/collector/receiver/nopreceiver v0.157.0
	go.opentelemetry.io/collector/receiver/otlpreceiver v0.157.0
	go.opentelemetry.io/collector/service v0.157.0
	golang.org/x/sys v0.47.0
)

require (
	bitbucket.org/atlassian/go-asap/v2 v2.15.3 // indirect
	cel.dev/expr v0.25.2 // indirect
	cloud.google.com/go v0.123.0 // indirect
	cloud.google.com/go/auth v0.20.0 // indirect
	cloud.google.com/go/auth/oauth2adapt v0.2.8 // indirect
	cloud.google.com/go/compute v1.63.0 // indirect
	cloud.google.com/go/compute/metadata v0.9.0 // indirect
	cloud.google.com/go/iam v1.11.0 // indirect
	cloud.google.com/go/logging v1.18.0 // indirect
	cloud.google.com/go/longrunning v1.0.0 // indirect
	cloud.google.com/go/monitoring v1.29.0 // indirect
	cloud.google.com/go/pubsub/v2 v2.6.0 // indirect
	cloud.google.com/go/secretmanager v1.20.0 // indirect
	cloud.google.com/go/spanner v1.91.0 // indirect
	cloud.google.com/go/storage v1.62.1 // indirect
	cloud.google.com/go/trace v1.16.0 // indirect
	code.cloudfoundry.org/clock v0.0.0-20180518195852-02e53af36e6c // indirect
	code.cloudfoundry.org/garden v0.0.0-20241023020423-a21e43a17f84 // indirect
	code.cloudfoundry.org/go-diodes v0.0.0-20241007161556-ec30366c7912 // indirect
	code.cloudfoundry.org/go-loggregator v7.4.0+incompatible // indirect
	code.cloudfoundry.org/lager/v3 v3.11.0 // indirect
	code.cloudfoundry.org/rfc5424 v0.0.0-20201103192249-000122071b78 // indirect
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/99designs/go-keychain v0.0.0-20191008050251-8e49817e8af4 // indirect
	github.com/99designs/keyring v1.2.2 // indirect
	github.com/AthenZ/athenz v1.12.13 // indirect
	github.com/Azure/azure-kusto-go/azkustodata v1.2.2 // indirect
	github.com/Azure/azure-kusto-go/azkustoingest v1.2.2 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.22.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.14.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/data/aztables v1.4.1 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.12.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/messaging/azeventhubs/v2 v2.0.2 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/monitor/query/azmetrics v1.3.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5 v5.7.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor v0.13.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v4 v4.3.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources/v4 v4.0.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions/v2 v2.0.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/storage/azblob v1.8.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/storage/azqueue v1.0.1 // indirect
	github.com/Azure/go-amqp v1.7.0 // indirect
	github.com/Azure/go-ntlmssp v0.1.1 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.7.2 // indirect
	github.com/BurntSushi/toml v1.6.0 // indirect
	github.com/ClickHouse/ch-go v0.73.0 // indirect
	github.com/ClickHouse/clickhouse-go/v2 v2.47.0 // indirect
	github.com/Code-Hex/go-generics-cache v1.5.1 // indirect
	github.com/DataDog/agent-payload/v5 v5.0.205 // indirect
	github.com/DataDog/datadog-agent/comp/api/api/def v0.81.0 // indirect
	github.com/DataDog/datadog-agent/comp/core/config v0.82.0-devel.0.20260624113434-509b872045c2 // indirect
	github.com/DataDog/datadog-agent/comp/core/delegatedauth v0.81.0 // indirect
	github.com/DataDog/datadog-agent/comp/core/flare/builder v0.81.0 // indirect
	github.com/DataDog/datadog-agent/comp/core/flare/types v0.81.0 // indirect
	github.com/DataDog/datadog-agent/comp/core/hostname/hostnameinterface v0.80.2 // indirect
	github.com/DataDog/datadog-agent/comp/core/log/def v0.81.0 // indirect
	github.com/DataDog/datadog-agent/comp/core/secrets/def v0.81.0 // indirect
	github.com/DataDog/datadog-agent/comp/core/secrets/noop-impl v0.81.0 // indirect
	github.com/DataDog/datadog-agent/comp/core/status v0.81.0 // indirect
	github.com/DataDog/datadog-agent/comp/core/tagger/def v0.81.0-devel // indirect
	github.com/DataDog/datadog-agent/comp/core/tagger/origindetection v0.81.0 // indirect
	github.com/DataDog/datadog-agent/comp/core/tagger/types v0.81.0 // indirect
	github.com/DataDog/datadog-agent/comp/core/tagger/utils v0.81.0 // indirect
	github.com/DataDog/datadog-agent/comp/core/telemetry v0.81.0 // indirect
	github.com/DataDog/datadog-agent/comp/def v0.81.0 // indirect
	github.com/DataDog/datadog-agent/comp/forwarder/defaultforwarder v0.82.0-devel.0.20260708210941-b0d9b5a19458 // indirect
	github.com/DataDog/datadog-agent/comp/forwarder/orchestrator/orchestratorinterface v0.81.0-devel // indirect
	github.com/DataDog/datadog-agent/comp/logs-library v0.80.2 // indirect
	github.com/DataDog/datadog-agent/comp/logs/agent/config v0.80.2 // indirect
	github.com/DataDog/datadog-agent/comp/otelcol/logsagentpipeline v0.80.2 // indirect
	github.com/DataDog/datadog-agent/comp/otelcol/logsagentpipeline/logsagentpipelineimpl v0.80.2 // indirect
	github.com/DataDog/datadog-agent/comp/otelcol/otlp/components/exporter/logsagentexporter v0.80.2 // indirect
	github.com/DataDog/datadog-agent/comp/otelcol/otlp/components/exporter/serializerexporter v0.83.0-devel.0.20260714134811-fee4bbf7ff73 // indirect
	github.com/DataDog/datadog-agent/comp/otelcol/otlp/components/metricsclient v0.80.2 // indirect
	github.com/DataDog/datadog-agent/comp/serializer/logscompression v0.80.2 // indirect
	github.com/DataDog/datadog-agent/comp/serializer/metricscompression v0.81.0-devel // indirect
	github.com/DataDog/datadog-agent/comp/trace/compression/def v0.81.0 // indirect
	github.com/DataDog/datadog-agent/comp/trace/compression/impl-gzip v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/aggregator/ckey v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/api v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/collector/check/defaults v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/config/basic v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/config/buildschema v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/config/create v0.82.0-devel.0.20260624113434-509b872045c2 // indirect
	github.com/DataDog/datadog-agent/pkg/config/env v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/config/helper v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/config/mock v0.82.0-devel.0.20260624113434-509b872045c2 // indirect
	github.com/DataDog/datadog-agent/pkg/config/model v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/config/nodetreemodel v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/config/setup v0.82.0-devel.0.20260624113434-509b872045c2 // indirect
	github.com/DataDog/datadog-agent/pkg/config/structure v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/config/utils v0.82.0-devel.0.20260708210941-b0d9b5a19458 // indirect
	github.com/DataDog/datadog-agent/pkg/fips v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/client v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/diagnostic v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/message v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/sources v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/status/statusinterface v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/status/utils v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/logs/types v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/metrics v0.82.0-devel.0.20260708210941-b0d9b5a19458 // indirect
	github.com/DataDog/datadog-agent/pkg/obfuscate v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/inframetadata v0.83.0-devel.0.20260714134811-fee4bbf7ff73 // indirect
	github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/attributes v0.83.0-devel.0.20260714134811-fee4bbf7ff73 // indirect
	github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/logs v0.83.0-devel.0.20260714134811-fee4bbf7ff73 // indirect
	github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/metrics v0.83.0-devel.0.20260714134811-fee4bbf7ff73 // indirect
	github.com/DataDog/datadog-agent/pkg/opentelemetry-mapping-go/otlp/rum v0.83.0-devel.0.20260714134811-fee4bbf7ff73 // indirect
	github.com/DataDog/datadog-agent/pkg/orchestrator/model v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/orchestrator/util v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/process/util/api v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/proto v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/remoteconfig/state v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/serializer v0.82.0-devel.0.20260708210941-b0d9b5a19458 // indirect
	github.com/DataDog/datadog-agent/pkg/status/health v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/tagger/types v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/tagset v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/template v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/trace v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/trace/log v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/trace/otel v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/trace/stats v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/trace/traceutil v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/util/backoff v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/util/buf v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/util/cgroups v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/util/compression v0.81.0-devel // indirect
	github.com/DataDog/datadog-agent/pkg/util/defaultpaths v0.81.0-devel.0.20260624113434-509b872045c2 // indirect
	github.com/DataDog/datadog-agent/pkg/util/executable v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/util/filesystem v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/util/fxutil v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/util/hostname/validate v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/util/hostport v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/util/http v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/util/json v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/util/log v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/util/option v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/util/otel v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/util/pointer v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/util/quantile v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/util/scrubber v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/util/sort v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/util/startstop v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/util/statstracker v0.80.2 // indirect
	github.com/DataDog/datadog-agent/pkg/util/system v0.81.0 // indirect
	github.com/DataDog/datadog-agent/pkg/util/winutil v0.82.0-devel.0.20260624113434-509b872045c2 // indirect
	github.com/DataDog/datadog-agent/pkg/version v0.81.0 // indirect
	github.com/DataDog/datadog-api-client-go/v2 v2.62.0 // indirect
	github.com/DataDog/datadog-go/v5 v5.9.0 // indirect
	github.com/DataDog/go-acl v1.0.1 // indirect
	github.com/DataDog/go-sqllexer v0.2.2 // indirect
	github.com/DataDog/go-tuf v1.1.1-0.5.2 // indirect
	github.com/DataDog/gohai v0.0.0-20230524154621-4316413895ee // indirect
	github.com/DataDog/mmh3 v0.0.0-20210722141835-012dc69a9e49 // indirect
	github.com/DataDog/sketches-go v1.4.8 // indirect
	github.com/DataDog/zstd v1.5.8-0.20260421145859-31a7e515a571 // indirect
	github.com/DataDog/zstd_0 v0.0.0-20210310093942-586c1286621f // indirect
	github.com/DeRuina/timberjack v1.4.5 // indirect
	github.com/GehirnInc/crypt v0.0.0-20230320061759-8cc1b52080c5 // indirect
	github.com/GoogleCloudPlatform/grpc-gcp-go/grpcgcp v1.6.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/detectors/gcp v1.34.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector v0.58.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/collector/googlemanagedprometheus v0.58.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/metric v0.57.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v1.34.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/extension/googleclientauthextension v0.58.0 // indirect
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/internal/resourcemapping v0.58.0 // indirect
	github.com/HdrHistogram/hdrhistogram-go v1.2.0 // indirect
	github.com/Khan/genqlient v0.8.1 // indirect
	github.com/KimMachineGun/automemlimit v0.7.5 // indirect
	github.com/Masterminds/semver/v3 v3.5.0 // indirect
	github.com/Microsoft/go-winio v0.6.2 // indirect
	github.com/RoaringBitmap/roaring/v2 v2.8.0 // indirect
	github.com/SAP/go-hdb v1.16.12 // indirect
	github.com/SermoDigital/jose v0.9.2-0.20180104203859-803625baeddc // indirect
	github.com/Showmax/go-fqdn v1.0.0 // indirect
	github.com/aerospike/aerospike-client-go/v8 v8.7.0 // indirect
	github.com/alecthomas/participle/v2 v2.1.4 // indirect
	github.com/alecthomas/units v0.0.0-20240927000941-0f3dac36c52b // indirect
	github.com/alexbrainman/sspi v0.0.0-20250919150558-7d374ff0d59e // indirect
	github.com/aliyun/aliyun-log-go-sdk v0.1.100 // indirect
	github.com/andybalholm/brotli v1.2.1 // indirect
	github.com/antchfx/xmlquery v1.5.1 // indirect
	github.com/antchfx/xpath v1.3.6 // indirect
	github.com/apache/arrow-go/v18 v18.6.0 // indirect
	github.com/apache/cassandra-gocql-driver/v2 v2.1.2 // indirect
	github.com/apache/pulsar-client-go v0.21.0 // indirect
	github.com/apache/thrift v0.24.0 // indirect
	github.com/apapsch/go-jsonmerge/v2 v2.0.0 // indirect
	github.com/ardielle/ardielle-go v1.5.2 // indirect
	github.com/armon/go-metrics v0.4.1 // indirect
	github.com/aws/aws-lambda-go v1.54.0 // indirect
	github.com/aws/aws-msk-iam-sasl-signer-go v1.0.4 // indirect
	github.com/aws/aws-sdk-go v1.55.8 // indirect
	github.com/aws/aws-sdk-go-v2 v1.42.1 // indirect
	github.com/aws/aws-sdk-go-v2/aws/protocol/eventstream v1.7.14 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.32.29 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.19.28 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.30 // indirect
	github.com/aws/aws-sdk-go-v2/feature/s3/manager v1.16.15 // indirect
	github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager v0.3.1 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.30 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.30 // indirect
	github.com/aws/aws-sdk-go-v2/internal/v4a v1.4.31 // indirect
	github.com/aws/aws-sdk-go-v2/service/cloudwatch v1.63.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs v1.79.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.316.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ecs v1.88.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/elasticache v1.52.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.13 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/checksum v1.9.23 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.30 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/s3shared v1.19.31 // indirect
	github.com/aws/aws-sdk-go-v2/service/kafka v1.52.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/kinesis v1.45.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/lightsail v1.54.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/rds v1.118.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/s3 v1.105.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/secretsmanager v1.43.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/servicediscovery v1.41.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/signin v1.4.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sqs v1.45.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.32.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.37.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.44.0 // indirect
	github.com/aws/aws-sdk-go-v2/service/xray v1.38.0 // indirect
	github.com/aws/smithy-go v1.27.3 // indirect
	github.com/axiomhq/hyperloglog v0.2.6 // indirect
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/bboreham/go-loser v0.0.0-20230920113527-fcc2c21820a3 // indirect
	github.com/beevik/ntp v1.5.0 // indirect
	github.com/benbjohnson/clock v1.3.5 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bits-and-blooms/bitset v1.12.0 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/bmatcuk/doublestar/v4 v4.10.0 // indirect
	github.com/bmizerany/pat v0.0.0-20210406213842-e4b6760bdd6f // indirect
	github.com/buger/jsonparser v1.2.0 // indirect
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cenkalti/backoff/v7 v7.0.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cihub/seelog v0.0.0-20170130134532-f561c5e57575 // indirect
	github.com/cilium/ebpf v0.22.0 // indirect
	github.com/cloudfoundry-incubator/uaago v0.0.0-20190307164349-8136b7bbe76e // indirect
	github.com/cloudfoundry/go-cfclient/v3 v3.0.0-beta.1 // indirect
	github.com/cncf/xds/go v0.0.0-20260202195803-dba9d589def2 // indirect
	github.com/codegangsta/inject v0.0.0-20150114235600-33e0aa1cb7c0 // indirect
	github.com/containerd/cgroups/v3 v3.1.3 // indirect
	github.com/containerd/containerd/api v1.10.0 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/errdefs/pkg v0.3.0 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/ttrpc v1.2.8 // indirect
	github.com/containerd/typeurl/v2 v2.2.3 // indirect
	github.com/coreos/go-oidc/v3 v3.20.0 // indirect
	github.com/coreos/go-systemd/v22 v22.7.0 // indirect
	github.com/cyphar/filepath-securejoin v0.6.1 // indirect
	github.com/danieljoos/wincred v1.2.3 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dennwc/varint v1.0.0 // indirect
	github.com/dgryski/go-metro v0.0.0-20250106013310-edb8663e5e33 // indirect
	github.com/digitalocean/go-metadata v0.0.0-20250129100319-e3650a3df44b // indirect
	github.com/digitalocean/godo v1.193.0 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/docker/go-connections v0.7.0 // indirect
	github.com/docker/go-units v0.5.0 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/dvsekhvalnov/jose2go v1.7.0 // indirect
	github.com/ebitengine/purego v0.10.1 // indirect
	github.com/edsrzf/mmap-go v1.2.1-0.20241212181136-fad1cd13edbd // indirect
	github.com/elastic/elastic-transport-go/v8 v8.8.0 // indirect
	github.com/elastic/go-docappender/v2 v2.14.1 // indirect
	github.com/elastic/go-freelru v0.16.0 // indirect
	github.com/elastic/go-grok v0.3.1 // indirect
	github.com/elastic/go-structform v0.0.12 // indirect
	github.com/elastic/lunes v0.2.2 // indirect
	github.com/emicklei/go-restful/v3 v3.13.0 // indirect
	github.com/emirpasic/gods v1.18.1 // indirect
	github.com/envoyproxy/go-control-plane/envoy v1.37.0 // indirect
	github.com/envoyproxy/protoc-gen-validate v1.3.3 // indirect
	github.com/euank/go-kmsg-parser v2.0.0+incompatible // indirect
	github.com/expr-lang/expr v1.17.8 // indirect
	github.com/facebook/time v0.0.0-20240510113249-fa89cc575891 // indirect
	github.com/facette/natsort v0.0.0-20181210072756-2cd4dd1e2dcb // indirect
	github.com/fatih/color v1.19.0 // indirect
	github.com/felixge/fgprof v0.9.5 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/foxboron/go-tpm-keyfiles v0.0.0-20251226215517-609e4778396f // indirect
	github.com/fsnotify/fsnotify v1.10.1 // indirect
	github.com/fxamacker/cbor/v2 v2.9.2 // indirect
	github.com/gabriel-vasile/mimetype v1.4.7 // indirect
	github.com/getsentry/sentry-go v0.48.0 // indirect
	github.com/go-asn1-ber/asn1-ber v1.5.8 // indirect
	github.com/go-faster/city v1.0.1 // indirect
	github.com/go-faster/errors v0.7.1 // indirect
	github.com/go-jose/go-jose/v4 v4.1.4 // indirect
	github.com/go-kit/kit v0.12.0 // indirect
	github.com/go-kit/log v0.2.1 // indirect
	github.com/go-ldap/ldap/v3 v3.4.14 // indirect
	github.com/go-logfmt/logfmt v0.6.1 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-martini/martini v0.0.0-20170121215854-22fa46961aab // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/go-openapi/analysis v0.25.0 // indirect
	github.com/go-openapi/errors v0.22.7 // indirect
	github.com/go-openapi/jsonpointer v0.23.1 // indirect
	github.com/go-openapi/jsonreference v0.21.5 // indirect
	github.com/go-openapi/loads v0.23.3 // indirect
	github.com/go-openapi/spec v0.22.4 // indirect
	github.com/go-openapi/strfmt v0.26.2 // indirect
	github.com/go-openapi/swag v0.25.5 // indirect
	github.com/go-openapi/swag/cmdutils v0.25.5 // indirect
	github.com/go-openapi/swag/conv v0.25.5 // indirect
	github.com/go-openapi/swag/fileutils v0.25.5 // indirect
	github.com/go-openapi/swag/jsonname v0.26.0 // indirect
	github.com/go-openapi/swag/jsonutils v0.25.5 // indirect
	github.com/go-openapi/swag/loading v0.25.5 // indirect
	github.com/go-openapi/swag/mangling v0.25.5 // indirect
	github.com/go-openapi/swag/netutils v0.25.5 // indirect
	github.com/go-openapi/swag/stringutils v0.25.5 // indirect
	github.com/go-openapi/swag/typeutils v0.25.5 // indirect
	github.com/go-openapi/swag/yamlutils v0.25.5 // indirect
	github.com/go-openapi/validate v0.25.2 // indirect
	github.com/go-resty/resty/v2 v2.17.2 // indirect
	github.com/go-sql-driver/mysql v1.10.0 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/go-viper/mapstructure/v2 v2.5.0 // indirect
	github.com/go-zookeeper/zk v1.0.4 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/goccy/go-json v0.10.6 // indirect
	github.com/goccy/go-yaml v1.19.2 // indirect
	github.com/godbus/dbus v0.0.0-20190726142602-4481cbc300e2 // indirect
	github.com/godbus/dbus/v5 v5.2.2 // indirect
	github.com/gofrs/flock v0.13.0 // indirect
	github.com/gofrs/uuid v4.4.0+incompatible // indirect
	github.com/gogo/googleapis v1.4.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.3.1 // indirect
	github.com/golang-sql/civil v0.0.0-20220223132316-b832511892a9 // indirect
	github.com/golang-sql/sqlexp v0.1.0 // indirect
	github.com/golang/groupcache v0.0.0-20241129210726-2c02b8208cf8 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/cadvisor v0.57.0 // indirect
	github.com/google/flatbuffers v25.12.19+incompatible // indirect
	github.com/google/gnostic-models v0.7.1 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/go-github/v88 v88.0.0 // indirect
	github.com/google/go-querystring v1.2.0 // indirect
	github.com/google/go-tpm v0.9.8 // indirect
	github.com/google/pprof v0.0.0-20260507013755-92041b743c96 // indirect
	github.com/google/s2a-go v0.1.9 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.17 // indirect
	github.com/googleapis/gax-go/v2 v2.23.0 // indirect
	github.com/gophercloud/gophercloud/v2 v2.12.0 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/gorilla/websocket v1.5.4-0.20250319132907-e064f32e3674 // indirect
	github.com/gosnmp/gosnmp v1.43.2 // indirect
	github.com/grafana/clusterurl v0.2.1 // indirect
	github.com/grafana/faro/pkg/go v0.0.0-20260427090633-bb5f9417df83 // indirect
	github.com/grafana/loki/pkg/push v0.0.0-20240514112848-a1b1eeb09583 // indirect
	github.com/grafana/regexp v0.0.0-20250905093917-f7b3be9d1853 // indirect
	github.com/grobie/gomemcache v0.0.0-20230213081705-239240bbc445 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.29.0 // indirect
	github.com/gsterjov/go-libsecret v0.0.0-20161001094733-a6f4afe4910c // indirect
	github.com/hamba/avro/v2 v2.31.0 // indirect
	github.com/hashicorp/consul/api v1.32.1 // indirect
	github.com/hashicorp/cronexpr v1.1.3 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-hclog v1.6.3 // indirect
	github.com/hashicorp/go-immutable-radix v1.3.1 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.8 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-uuid v1.0.3 // indirect
	github.com/hashicorp/go-version v1.9.0 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/hashicorp/golang-lru/v2 v2.0.7 // indirect
	github.com/hashicorp/nomad/api v0.0.0-20260528135333-5b027732945f // indirect
	github.com/hashicorp/serf v0.10.1 // indirect
	github.com/hetznercloud/hcloud-go/v2 v2.44.0 // indirect
	github.com/huandu/go-clone v1.7.3 // indirect
	github.com/huaweicloud/huaweicloud-sdk-go-v3 v0.1.202 // indirect
	github.com/iancoleman/strcase v0.3.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/influxdata/influxdb-observability/common v0.5.12 // indirect
	github.com/influxdata/influxdb-observability/influx2otel v0.5.12 // indirect
	github.com/influxdata/influxdb-observability/otel2influx v0.5.12 // indirect
	github.com/influxdata/line-protocol/v2 v2.2.1 // indirect
	github.com/ionos-cloud/sdk-go/v6 v6.3.7 // indirect
	github.com/itchyny/timefmt-go v0.1.8 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/pgx/v5 v5.10.0 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/jaegertracing/jaeger-idl v0.9.0 // indirect
	github.com/jcmturner/aescts/v2 v2.0.0 // indirect
	github.com/jcmturner/dnsutils/v2 v2.0.0 // indirect
	github.com/jcmturner/gofork v1.7.6 // indirect
	github.com/jcmturner/goidentity/v6 v6.0.1 // indirect
	github.com/jcmturner/gokrb5/v8 v8.4.4 // indirect
	github.com/jcmturner/rpc/v2 v2.0.3 // indirect
	github.com/jellydator/ttlcache/v3 v3.4.1 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/jonboulle/clockwork v0.5.0 // indirect
	github.com/jpillora/backoff v1.0.0 // indirect
	github.com/json-iterator/go v1.1.13-0.20220915233716-71ac16282d12 // indirect
	github.com/julienschmidt/httprouter v1.3.0 // indirect
	github.com/kamstrup/intmap v0.5.2 // indirect
	github.com/klauspost/compress v1.19.0 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/knadh/koanf/maps v0.1.2 // indirect
	github.com/knadh/koanf/providers/confmap v1.0.0 // indirect
	github.com/knadh/koanf/v2 v2.3.5 // indirect
	github.com/kolo/xmlrpc v0.0.0-20220921171641-a4b6fa1dd06b // indirect
	github.com/kr/fs v0.1.0 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/leodido/go-syslog/v4 v4.6.0 // indirect
	github.com/leodido/ragel-machinery v0.0.0-20190525184631-5f46317e436b // indirect
	github.com/lestrrat-go/strftime v1.2.0 // indirect
	github.com/lib/pq v1.12.3 // indirect
	github.com/libp2p/go-reuseport v0.4.0 // indirect
	github.com/lightstep/go-expohisto v1.0.0 // indirect
	github.com/linkedin/goavro/v2 v2.15.0 // indirect
	github.com/linode/go-metadata v0.2.4 // indirect
	github.com/linode/linodego v1.69.1 // indirect
	github.com/logicmonitor/lm-data-sdk-go v1.3.4 // indirect
	github.com/lufia/plan9stats v0.0.0-20260330125221-c963978e514e // indirect
	github.com/magefile/mage v1.15.0 // indirect
	github.com/mailru/easyjson v0.9.0 // indirect
	github.com/martini-contrib/render v0.0.0-20150707142108-ec18f8345a11 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.22 // indirect
	github.com/mdlayher/socket v0.6.0 // indirect
	github.com/mdlayher/vsock v1.3.0 // indirect
	github.com/michel-laterman/proxy-connect-dialer-go v0.1.0 // indirect
	github.com/microsoft/ApplicationInsights-Go v0.4.4 // indirect
	github.com/microsoft/go-mssqldb v1.9.6 // indirect
	github.com/miekg/dns v1.1.72 // indirect
	github.com/minio/sha256-simd v1.0.1 // indirect
	github.com/mistifyio/go-zfs v2.1.2-0.20190413222219-f784269be439+incompatible // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/hashstructure/v2 v2.0.2 // indirect
	github.com/mitchellh/mapstructure v1.5.1-0.20231216201459-8508981c8b6c // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/docker-image-spec v1.3.1 // indirect
	github.com/moby/moby/api v1.55.0 // indirect
	github.com/moby/moby/client v0.5.0 // indirect
	github.com/moby/sys/mountinfo v0.7.2 // indirect
	github.com/moby/sys/userns v0.1.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826 // indirect
	github.com/mongodb-forks/digest v1.1.0 // indirect
	github.com/mschoch/smat v0.2.0 // indirect
	github.com/mtibben/percent v0.2.1 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/mwitkow/go-conntrack v0.0.0-20190716064945-2f068394615f // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/netsampler/goflow2/v2 v2.2.6 // indirect
	github.com/nginx/nginx-prometheus-exporter v1.5.1 // indirect
	github.com/oapi-codegen/runtime v1.3.1 // indirect
	github.com/oklog/ulid/v2 v2.1.1 // indirect
	github.com/onsi/ginkgo/v2 v2.27.5 // indirect
	github.com/open-telemetry/opamp-go v0.23.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/internal/basicauth v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/internal/credentialsfile v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/opampcustommessages v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/awsutil v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/containerinsight v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/cwlogs v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/ecsutil v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/k8s v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/metrics v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/proxy v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/xray v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/collectd v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/datadog v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/exp/metrics v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/gopsutilenv v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/grpcutil v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/healthcheck v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sinventory v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/kafka v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/kubelet v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/otelarrow v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/pdatautil v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/rabbitmq v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/sharedcomponent v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/sqlquery v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/core/xidutils v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/experimentalmetricmetadata v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/topic v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/resourcetotelemetry v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/sampling v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/stanza v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/status v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azure v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/azurelogs v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/faro v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/loki v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/pprof v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheusremotewrite v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/signalfx v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/skywalking v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/splunk v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/zipkin v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/winperfcounters v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xstreamencoding v0.157.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/scraper/zookeeperscraper v0.157.0 // indirect
	github.com/open-telemetry/otel-arrow/go v0.49.0 // indirect
	github.com/opencontainers/cgroups v0.0.6 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/opencontainers/runtime-spec v1.3.0 // indirect
	github.com/opensearch-project/opensearch-go/v4 v4.6.0 // indirect
	github.com/openshift/api v0.0.0-20251015095338-264e80a2b6e7 // indirect
	github.com/openshift/client-go v0.0.0-20251015124057-db0dee36e235 // indirect
	github.com/openzipkin/zipkin-go v0.4.3 // indirect
	github.com/orcaman/concurrent-map/v2 v2.0.1 // indirect
	github.com/oschwald/geoip2-golang/v2 v2.2.0 // indirect
	github.com/oschwald/maxminddb-golang/v2 v2.3.0 // indirect
	github.com/outcaste-io/ristretto v0.2.3 // indirect
	github.com/outscale/osc-sdk-go/v2 v2.34.0 // indirect
	github.com/ovh/go-ovh v1.9.0 // indirect
	github.com/oxtoacart/bpool v0.0.0-20190530202638-03653db5a59c // indirect
	github.com/parquet-go/bitpack v1.0.0 // indirect
	github.com/parquet-go/jsonlite v1.0.0 // indirect
	github.com/parquet-go/parquet-go v0.30.1 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/paulmach/orb v0.13.0 // indirect
	github.com/pavlo-v-chernykh/keystore-go/v4 v4.5.0 // indirect
	github.com/pb33f/jsonpath v0.8.2 // indirect
	github.com/pb33f/libopenapi v0.37.2 // indirect
	github.com/pb33f/ordered-map/v2 v2.3.1 // indirect
	github.com/pbnjay/memory v0.0.0-20210728143218-7b4eea64cf58 // indirect
	github.com/philhofer/fwd v1.2.0 // indirect
	github.com/pierrec/lz4 v2.6.1+incompatible // indirect
	github.com/pierrec/lz4/v4 v4.1.27 // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pkg/sftp v1.13.11 // indirect
	github.com/planetscale/vtprotobuf v0.6.1-0.20240319094008-0393e58bdf10 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/pquerna/cachecontrol v0.1.0 // indirect
	github.com/prometheus/alertmanager v0.32.1 // indirect
	github.com/prometheus/client_golang v1.23.3-0.20251103151724-a5ae20370e5e // indirect
	github.com/prometheus/client_golang/exp v0.0.0-20260518105423-c9d5bc4c50a9 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.70.0 // indirect
	github.com/prometheus/common/assets v0.2.0 // indirect
	github.com/prometheus/exporter-toolkit v0.17.1 // indirect
	github.com/prometheus/otlptranslator v1.0.0 // indirect
	github.com/prometheus/procfs v0.21.1 // indirect
	github.com/prometheus/prometheus v0.312.0 // indirect
	github.com/prometheus/sigv4 v0.4.1 // indirect
	github.com/puzpuzpuz/xsync/v4 v4.5.0 // indirect
	github.com/rabbitmq/amqp091-go v1.12.0 // indirect
	github.com/rdforte/gomaxecs v1.1.2 // indirect
	github.com/redis/go-redis/v9 v9.21.0 // indirect
	github.com/relvacode/iso8601 v1.7.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/richardartoul/molecule v1.0.1-0.20240531184615-7ca0df43c0b3 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/samber/lo v1.52.0 // indirect
	github.com/scaleway/scaleway-sdk-go v1.0.0-beta.36 // indirect
	github.com/scalyr/dataset-go v0.21.0 // indirect
	github.com/secure-systems-lab/go-securesystemslib v0.11.0 // indirect
	github.com/segmentio/asm v1.2.1 // indirect
	github.com/shirou/gopsutil/v3 v3.24.5 // indirect
	github.com/shirou/gopsutil/v4 v4.26.6 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/shurcooL/httpfs v0.0.0-20230704072500-f1e31cf0ba5c // indirect
	github.com/signalfx/com_signalfx_metrics_protobuf v0.0.3 // indirect
	github.com/signalfx/sapm-proto v0.18.0 // indirect
	github.com/sijms/go-ora/v2 v2.9.0 // indirect
	github.com/sirupsen/logrus v1.9.4 // indirect
	github.com/snowflakedb/gosnowflake/v2 v2.1.0 // indirect
	github.com/spaolacci/murmur3 v1.1.0 // indirect
	github.com/spf13/cast v1.10.0 // indirect
	github.com/spf13/cobra v1.10.2 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/spiffe/go-spiffe/v2 v2.6.0 // indirect
	github.com/splunk/stef/go/grpc v0.1.2 // indirect
	github.com/splunk/stef/go/otel v0.1.2 // indirect
	github.com/splunk/stef/go/pdata v0.1.2 // indirect
	github.com/splunk/stef/go/pkg v0.1.2 // indirect
	github.com/stackitcloud/stackit-sdk-go/core v0.26.0 // indirect
	github.com/stretchr/objx v0.5.3 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	github.com/tedsuo/rata v1.0.0 // indirect
	github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common v1.3.132 // indirect
	github.com/tg123/go-htpasswd v1.2.5 // indirect
	github.com/thda/tds v0.1.7 // indirect
	github.com/tidwall/gjson v1.19.0 // indirect
	github.com/tidwall/match v1.2.0 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/tinylru v1.2.1 // indirect
	github.com/tidwall/wal v1.2.1 // indirect
	github.com/tilinna/clock v1.1.0 // indirect
	github.com/tinylib/msgp v1.6.4 // indirect
	github.com/tjfoc/gmsm v1.4.1 // indirect
	github.com/tklauser/go-sysconf v0.4.0 // indirect
	github.com/tklauser/numcpus v0.12.0 // indirect
	github.com/twmb/franz-go v1.21.5 // indirect
	github.com/twmb/franz-go/pkg/kadm v1.18.0 // indirect
	github.com/twmb/franz-go/pkg/kmsg v1.13.1 // indirect
	github.com/twmb/franz-go/pkg/sasl/kerberos v1.1.0 // indirect
	github.com/twmb/franz-go/plugin/kzap v1.1.2 // indirect
	github.com/twmb/murmur3 v1.1.8 // indirect
	github.com/twpayne/go-geom v1.6.1 // indirect
	github.com/ua-parser/uap-go v0.0.0-20251207011819-db9adb27a0b8 // indirect
	github.com/valyala/fastjson v1.6.10 // indirect
	github.com/vektah/gqlparser/v2 v2.5.22 // indirect
	github.com/vincent-petithory/dataurl v1.0.0 // indirect
	github.com/vmihailenco/msgpack/v5 v5.4.1 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	github.com/vmware/go-vmware-nsxt v0.0.0-20230223012718-d31b8a1ca05e // indirect
	github.com/vmware/govmomi v0.55.1 // indirect
	github.com/vultr/govultr/v3 v3.31.2 // indirect
	github.com/wadey/gocovmerge v0.0.0-20160331181800-b5bfa59ec0ad // indirect
	github.com/wk8/go-ordered-map/v2 v2.1.8 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	github.com/xdg-go/pbkdf2 v1.0.0 // indirect
	github.com/xdg-go/scram v1.2.0 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	github.com/youmark/pkcs8 v0.0.0-20240726163527-a2c0da244d78 // indirect
	github.com/yuin/gopher-lua v1.1.2 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	github.com/zeebo/xxh3 v1.1.0 // indirect
	gitlab.com/gitlab-org/api/client-go/v2 v2.47.0 // indirect
	go.elastic.co/fastjson v1.5.1 // indirect
	go.etcd.io/bbolt v1.5.0 // indirect
	go.mongodb.org/atlas v0.38.0 // indirect
	go.mongodb.org/mongo-driver v1.17.7 // indirect
	go.mongodb.org/mongo-driver/v2 v2.8.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/collector v0.157.0 // indirect
	go.opentelemetry.io/collector/client v1.63.0 // indirect
	go.opentelemetry.io/collector/component/componentstatus v0.157.0 // indirect
	go.opentelemetry.io/collector/component/componenttest v0.157.0 // indirect
	go.opentelemetry.io/collector/config/configauth v1.63.0 // indirect
	go.opentelemetry.io/collector/config/configcompression v1.63.0 // indirect
	go.opentelemetry.io/collector/config/configgrpc v0.157.0 // indirect
	go.opentelemetry.io/collector/config/confighttp v0.157.0 // indirect
	go.opentelemetry.io/collector/config/configmiddleware v1.63.0 // indirect
	go.opentelemetry.io/collector/config/confignet v1.63.0 // indirect
	go.opentelemetry.io/collector/config/configopaque v1.63.0 // indirect
	go.opentelemetry.io/collector/config/configoptional v1.63.0 // indirect
	go.opentelemetry.io/collector/config/configretry v1.63.0 // indirect
	go.opentelemetry.io/collector/config/configtelemetry v0.157.0 // indirect
	go.opentelemetry.io/collector/config/configtls v1.63.0 // indirect
	go.opentelemetry.io/collector/confmap/xconfmap v0.157.0 // indirect
	go.opentelemetry.io/collector/connector/connectortest v0.157.0 // indirect
	go.opentelemetry.io/collector/connector/xconnector v0.157.0 // indirect
	go.opentelemetry.io/collector/consumer v1.63.0 // indirect
	go.opentelemetry.io/collector/consumer/consumererror v0.157.0 // indirect
	go.opentelemetry.io/collector/consumer/consumererror/xconsumererror v0.157.0 // indirect
	go.opentelemetry.io/collector/consumer/consumertest v0.157.0 // indirect
	go.opentelemetry.io/collector/consumer/xconsumer v0.157.0 // indirect
	go.opentelemetry.io/collector/exporter/exporterhelper v0.157.0 // indirect
	go.opentelemetry.io/collector/exporter/exporterhelper/xexporterhelper v0.157.0 // indirect
	go.opentelemetry.io/collector/exporter/exportertest v0.157.0 // indirect
	go.opentelemetry.io/collector/exporter/xexporter v0.157.0 // indirect
	go.opentelemetry.io/collector/extension/extensionauth v1.63.0 // indirect
	go.opentelemetry.io/collector/extension/extensioncapabilities v0.157.0 // indirect
	go.opentelemetry.io/collector/extension/extensionmiddleware v0.157.0 // indirect
	go.opentelemetry.io/collector/extension/extensiontest v0.157.0 // indirect
	go.opentelemetry.io/collector/extension/xextension v0.157.0 // indirect
	go.opentelemetry.io/collector/featuregate v1.63.0 // indirect
	go.opentelemetry.io/collector/filter v0.157.0 // indirect
	go.opentelemetry.io/collector/internal/componentalias v0.157.0 // indirect
	go.opentelemetry.io/collector/internal/fanoutconsumer v0.157.0 // indirect
	go.opentelemetry.io/collector/internal/memorylimiter v0.157.0 // indirect
	go.opentelemetry.io/collector/internal/sharedcomponent v0.157.0 // indirect
	go.opentelemetry.io/collector/internal/telemetry v0.157.0 // indirect
	go.opentelemetry.io/collector/pdata v1.63.0 // indirect
	go.opentelemetry.io/collector/pdata/pprofile v0.157.0 // indirect
	go.opentelemetry.io/collector/pdata/testdata v0.157.0 // indirect
	go.opentelemetry.io/collector/pdata/xpdata v0.157.0 // indirect
	go.opentelemetry.io/collector/pipeline v1.63.0 // indirect
	go.opentelemetry.io/collector/pipeline/xpipeline v0.157.0 // indirect
	go.opentelemetry.io/collector/processor/processorhelper v0.157.0 // indirect
	go.opentelemetry.io/collector/processor/processorhelper/xprocessorhelper v0.157.0 // indirect
	go.opentelemetry.io/collector/processor/processortest v0.157.0 // indirect
	go.opentelemetry.io/collector/processor/xprocessor v0.157.0 // indirect
	go.opentelemetry.io/collector/receiver/receiverhelper v0.157.0 // indirect
	go.opentelemetry.io/collector/receiver/receivertest v0.157.0 // indirect
	go.opentelemetry.io/collector/receiver/xreceiver v0.157.0 // indirect
	go.opentelemetry.io/collector/scraper v0.157.0 // indirect
	go.opentelemetry.io/collector/scraper/scraperhelper v0.157.0 // indirect
	go.opentelemetry.io/collector/semconv v0.128.1-0.20250610090210-188191247685 // indirect
	go.opentelemetry.io/collector/service/hostcapabilities v0.157.0 // indirect
	go.opentelemetry.io/contrib/bridges/otelzap v0.19.0 // indirect
	go.opentelemetry.io/contrib/detectors/gcp v1.43.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.69.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/httptrace/otelhttptrace v0.69.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.69.0 // indirect
	go.opentelemetry.io/contrib/otelconf v0.24.0 // indirect
	go.opentelemetry.io/contrib/propagators/autoprop v0.69.0 // indirect
	go.opentelemetry.io/contrib/propagators/aws v1.44.0 // indirect
	go.opentelemetry.io/contrib/propagators/b3 v1.44.0 // indirect
	go.opentelemetry.io/contrib/propagators/jaeger v1.44.0 // indirect
	go.opentelemetry.io/contrib/propagators/ot v1.44.0 // indirect
	go.opentelemetry.io/contrib/zpages v0.69.0 // indirect
	go.opentelemetry.io/ebpf-profiler v0.0.202627 // indirect
	go.opentelemetry.io/otel v1.44.1-0.20260622141720-fbe3d073ba93 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc v0.20.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.20.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc v1.44.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.44.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.44.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.44.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.44.0 // indirect
	go.opentelemetry.io/otel/exporters/prometheus v0.66.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutlog v0.20.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdoutmetric v1.44.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.44.0 // indirect
	go.opentelemetry.io/otel/log v0.20.0 // indirect
	go.opentelemetry.io/otel/metric v1.44.1-0.20260622141720-fbe3d073ba93 // indirect
	go.opentelemetry.io/otel/schema v0.0.17 // indirect
	go.opentelemetry.io/otel/sdk v1.44.0 // indirect
	go.opentelemetry.io/otel/sdk/log v0.20.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.44.1-0.20260622141720-fbe3d073ba93 // indirect
	go.opentelemetry.io/otel/trace v1.44.1-0.20260622141720-fbe3d073ba93 // indirect
	go.opentelemetry.io/proto/otlp v1.10.0 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	go.uber.org/automaxprocs v1.6.0 // indirect
	go.uber.org/dig v1.19.0 // indirect
	go.uber.org/fx v1.24.0 // indirect
	go.uber.org/goleak v1.3.0 // indirect
	go.uber.org/mock v0.6.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.28.0 // indirect
	go.uber.org/zap/exp v0.3.0 // indirect
	go.yaml.in/yaml/v2 v2.4.4 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	go.yaml.in/yaml/v4 v4.0.0-rc.4 // indirect
	golang.org/x/crypto v0.54.0 // indirect
	golang.org/x/exp v0.0.0-20260611194520-c48552f49976 // indirect
	golang.org/x/mod v0.38.0 // indirect
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/oauth2 v0.36.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/term v0.45.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	golang.org/x/tools v0.48.0 // indirect
	gonum.org/v1/gonum v0.17.0 // indirect
	google.golang.org/api v0.287.1 // indirect
	google.golang.org/genproto v0.0.0-20260519071638-aa98bba5eb94 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20260630182238-925bb5da69e7 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260630182238-925bb5da69e7 // indirect
	google.golang.org/grpc v1.82.1 // indirect
	google.golang.org/protobuf v1.36.12-0.20260116114154-8c4c4ae446ca // indirect
	gopkg.in/evanphx/json-patch.v4 v4.13.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/ini.v1 v1.67.2 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.2.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/api v0.35.4 // indirect
	k8s.io/apimachinery v0.35.6 // indirect
	k8s.io/client-go v0.35.4 // indirect
	k8s.io/klog/v2 v2.140.0 // indirect
	k8s.io/kube-openapi v0.0.0-20260330154417-16be699c7b31 // indirect
	k8s.io/kubelet v0.35.4 // indirect
	k8s.io/utils v0.0.0-20260319190234-28399d86e0b5 // indirect
	modernc.org/b/v2 v2.1.11 // indirect
	modernc.org/libc v1.73.4 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
	modernc.org/sqlite v1.53.0 // indirect
	sigs.k8s.io/controller-runtime v0.23.3 // indirect
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v6 v6.3.2 // indirect
	sigs.k8s.io/yaml v1.6.0 // indirect
	skywalking.apache.org/repo/goapi v0.0.0-20240104145220-ba7202308dd4 // indirect
	software.sslmate.com/src/go-pkcs12 v0.7.3 // indirect
)
