module github.com/open-telemetry/opentelemetry-collector-contrib

go 1.13

require (
	github.com/client9/misspell v0.3.4
	github.com/google/addlicense v0.0.0-20190907113143-be125746c2c4
	github.com/open-telemetry/opentelemetry-collector v0.2.4-0.20200118022335-34b2459597f1
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter v0.0.0-20200116182905-41c032071dce
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kinesisexporter v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stackdriverexporter v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sprocessor v0.0.0-20200116182905-41c032071dce
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinscribereceiver v0.0.0
	github.com/pavius/impi v0.0.0-20180302134524-c1cbdcb8df2b
	golang.org/x/lint v0.0.0-20191125180803-fdd1cda4f05f
	golang.org/x/tools v0.0.0-20191205225056-3393d29bb9fe
	honnef.co/go/tools v0.0.1-2019.2.3
)

replace git.apache.org/thrift.git v0.12.0 => github.com/apache/thrift v0.12.0

// Replace references to modules that are in this repository with their relateive paths
// so that we always build with current (latest) version of the source code.

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsxrayexporter => ./exporter/awsxrayexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter => ./exporter/azuremonitorexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kinesisexporter => ./exporter/kinesisexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter => ./exporter/sapmexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter => ./exporter/signalfxexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stackdriverexporter => ./exporter/stackdriverexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver => ./receiver/collectdreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver => ./receiver/sapmreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver => ./receiver/signalfxreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinscribereceiver => ./receiver/zipkinscribereceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sprocessor => ./processor/k8sprocessor/

replace k8s.io/client-go => k8s.io/client-go v0.0.0-20190620085101-78d2af792bab
