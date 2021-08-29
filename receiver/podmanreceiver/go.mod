module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/podmanreceiver

go 1.16

require (
	github.com/StackExchange/wmi v1.2.1 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/scraperhelper v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.33.1-0.20210827152330-09258f969908
	go.uber.org/zap v1.19.0
	golang.org/x/tools v0.1.3 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/scraperhelper => ../scraperhelper

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/scrapererror => ../scrapererror
