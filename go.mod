module github.com/open-telemetry/opentelemetry-collector-contrib

go 1.12

require (
	github.com/client9/misspell v0.3.4
	github.com/google/addlicense v0.0.0-20190907113143-be125746c2c4
	github.com/open-telemetry/opentelemetry-collector v0.2.1-0.20191217213608-3cac55bf869f
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter v0.0.0-20191213162202-55b8658da81a
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kinesisexporter v0.0.0-20191213162202-55b8658da81a
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sapmexporter v0.0.0-20191216215953-d880d667b576
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter v0.0.0-20191216215953-d880d667b576
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stackdriverexporter v0.0.0-20191126142441-b2a048090ad6
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver v0.0.0-20191209163404-28d5712f4129
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver v0.0.0-20191216215953-d880d667b576
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/signalfxreceiver v0.0.0-20191216215953-d880d667b576
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinscribereceiver v0.0.0-20191126142441-b2a048090ad6
	github.com/open-telemetry/opentelemetry-collector-contrib/testbed v0.0.0-20191216203641-fdca8852f98c // indirect
	github.com/pavius/impi v0.0.0-20180302134524-c1cbdcb8df2b
	golang.org/x/lint v0.0.0-20191125180803-fdd1cda4f05f
	golang.org/x/tools v0.0.0-20191205225056-3393d29bb9fe
	honnef.co/go/tools v0.0.1-2019.2.3
)
