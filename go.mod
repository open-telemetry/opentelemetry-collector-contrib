module github.com/open-telemetry/opentelemetry-collector-contrib

go 1.12

replace github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter v0.0.0 => ./exporter/azuremonitorexporter

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver => ./receiver/collectdreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver => ./receiver/sapmreceiver

require (
	github.com/client9/misspell v0.3.4
	github.com/google/addlicense v0.0.0-20190907113143-be125746c2c4
	github.com/open-telemetry/opentelemetry-collector v0.2.1-0.20191205151336-8e2473c5e754
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/signalfxexporter v0.0.0-20191203211755-8ae89debd6c5
	github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stackdriverexporter v0.0.0-20191126142441-b2a048090ad6
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sapmreceiver v0.0.0
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinscribereceiver v0.0.0-20191126142441-b2a048090ad6
	github.com/pavius/impi v0.0.0-20180302134524-c1cbdcb8df2b
	golang.org/x/lint v0.0.0-20191125180803-fdd1cda4f05f
	golang.org/x/tools v0.0.0-20191205225056-3393d29bb9fe
	honnef.co/go/tools v0.0.1-2019.2.3
)
