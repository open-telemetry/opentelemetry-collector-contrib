module github.com/open-telemetry/opentelemetry-collector-contrib

go 1.14

require (
	github.com/fatih/color v1.10.0 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudlogserviceexporter v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsprometheusremotewriteexporter v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/carbonexporter v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dynatraceexporter v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticexporter v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/f5cloudexporter v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombexporter v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/jaegerthrifthttpexporter v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logzioexporter v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/newrelicexporter v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sentryexporter v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stackdriverexporter v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/httpforwarder v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/hostobserver v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sprocessor v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/routingprocessor v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanmetricsprocessor v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexecreceiver v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/stanzareceiver v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/wavefrontreceiver v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsperfcountersreceiver v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zookeeperreceiver v0.0.0-00010101000000-000000000000
	github.com/sirupsen/logrus v1.7.0 // indirect
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.19.1-0.20210127225953-68c5961f7bc2
	golang.org/x/sys v0.0.0-20210104204734-6f8348627aad
)

// Replace references to modules that are in this repository with their relateive paths
// so that we always build with current (latest) version of the source code.

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ./internal/common

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk => ./internal/splunk

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig => ./internal/k8sconfig

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/awsxray => ./internal/awsxray

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/alibabacloudlogserviceexporter => ./exporter/alibabacloudlogserviceexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsprometheusremotewriteexporter => ./exporter/awsprometheusremotewriteexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter => ./exporter/awsemfexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter => ./exporter/awsxrayexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter => ./exporter/azuremonitorexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/carbonexporter => ./exporter/carbonexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter => ./exporter/datadogexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/dynatraceexporter => ./exporter/dynatraceexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/f5cloudexporter => ./exporter/f5cloudexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombexporter => ./exporter/honeycombexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/jaegerthrifthttpexporter => ./exporter/jaegerthrifthttpexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/loadbalancingexporter => ./exporter/loadbalancingexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/newrelicexporter => ./exporter/newrelicexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awskinesisexporter => ./exporter/awskinesisexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logzioexporter => ./exporter/logzioexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter => ./exporter/sapmexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sentryexporter => ./exporter/sentryexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter => ./exporter/signalfxexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter => ./exporter/splunkhecexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stackdriverexporter => ./exporter/stackdriverexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sumologicexporter => ./exporter/sumologicexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticexporter => ./exporter/elasticexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/httpforwarder => ./extension/httpforwarder

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer => ./extension/observer

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/hostobserver => ./extension/observer/hostobserver

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver => ./extension/observer/k8sobserver

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchperresourceattr => ./pkg/batchperresourceattr

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpertrace => ./pkg/batchpertrace

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsecscontainermetricsreceiver => ./receiver/awsecscontainermetricsreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsxrayreceiver => ./receiver/awsxrayreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver => ./receiver/carbonreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver => ./receiver/collectdreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/statsdreceiver => ./receiver/statsdreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kubeletstatsreceiver => ./receiver/kubeletstatsreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver => ./receiver/redisreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator => ./receiver/receivercreator

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver => ./receiver/sapmreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver => ./receiver/k8sclusterreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver => ./receiver/signalfxreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver => ./receiver/splunkhecreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver => ./receiver/simpleprometheusreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/prometheusexecreceiver => ./receiver/prometheusexecreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/wavefrontreceiver => ./receiver/wavefrontreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver => ./receiver/dockerstatsreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsperfcountersreceiver => ./receiver/windowsperfcountersreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver => ./receiver/jmxreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zookeeperreceiver => ./receiver/zookeeperreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/stanzareceiver => ./receiver/stanzareceiver/

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/memcachedreceiver => ./receiver/memcachedreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/groupbytraceprocessor => ./processor/groupbytraceprocessor

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sprocessor => ./processor/k8sprocessor/

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor => ./processor/resourcedetectionprocessor/

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstransformprocessor => ./processor/metricstransformprocessor/

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/routingprocessor => ./processor/routingprocessor/

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor => ./processor/tailsamplingprocessor

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/spanmetricsprocessor => ./processor/spanmetricsprocessor/
