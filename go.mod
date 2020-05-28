module github.com/open-telemetry/opentelemetry-collector-contrib

go 1.14

require (
	github.com/client9/misspell v0.3.4
	github.com/golangci/golangci-lint v1.24.0
	github.com/google/addlicense v0.0.0-20200301095109-7c013a14f2e2
	github.com/jstemmer/go-junit-report v0.9.1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/carbonexporter v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombexporter v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/jaegerthrifthttpexporter v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kinesisexporter v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lightstepexporter v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stackdriverexporter v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sprocessor v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerlegacyreceiver v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/wavefrontreceiver v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinscribereceiver v0.0.0
	github.com/pavius/impi v0.0.0-20180302134524-c1cbdcb8df2b
	github.com/tcnksm/ghr v0.13.0
	go.opentelemetry.io/collector v0.3.1-0.20200522130256-daf2fc71ac65
	honnef.co/go/tools v0.0.1-2020.1.3
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter v0.0.0
)

replace git.apache.org/thrift.git v0.12.0 => github.com/apache/thrift v0.12.0

replace github.com/apache/thrift => github.com/apache/thrift v0.0.0-20161221203622-b2a4d4ae21c7

// Replace references to modules that are in this repository with their relateive paths
// so that we always build with current (latest) version of the source code.

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ./internal/common

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter => ./exporter/awsxrayexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter => ./exporter/azuremonitorexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/carbonexporter => ./exporter/carbonexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/honeycombexporter => ./exporter/honeycombexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/jaegerthrifthttpexporter => ./exporter/jaegerthrifthttpexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/lightstepexporter => ./exporter/lightstepexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kinesisexporter => ./exporter/kinesisexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter => ./exporter/sapmexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter => ./exporter/signalfxexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stackdriverexporter => ./exporter/stackdriverexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer => ./extension/observer

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/observer/k8sobserver => ./extension/observer/k8sobserver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver => ./receiver/carbonreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver => ./receiver/collectdreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerlegacyreceiver => ./receiver/jaegerlegacyreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/redisreceiver => ./receiver/redisreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/receivercreator => ./receiver/receivercreator

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver => ./receiver/sapmreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sclusterreceiver => ./receiver/k8sclusterreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver => ./receiver/signalfxreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver => ./receiver/simpleprometheusreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/wavefrontreceiver => ./receiver/wavefrontreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinscribereceiver => ./receiver/zipkinscribereceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sprocessor => ./processor/k8sprocessor/

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter => ./exporter/awsemfexporter

replace k8s.io/client-go => k8s.io/client-go v0.0.0-20190620085101-78d2af792bab

replace go.opentelemetry.io/collector => ../opentelemetry-collector
