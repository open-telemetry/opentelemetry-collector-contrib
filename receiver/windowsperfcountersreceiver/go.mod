module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/windowsperfcountersreceiver

go 1.16

require (
	github.com/StackExchange/wmi v1.2.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/scraperhelper v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.33.1-0.20210827152330-09258f969908
	go.opentelemetry.io/collector/model v0.33.1-0.20210827152330-09258f969908
	go.uber.org/zap v1.19.0
	golang.org/x/sys v0.0.0-20210615035016-665e8c7367d1
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/scraperhelper => ../scraperhelper

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/scrapererror => ../scrapererror
