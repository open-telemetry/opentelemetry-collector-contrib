module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver

go 1.16

require (
	github.com/leoluk/perflib_exporter v0.1.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/scrapererror v0.0.0-00010101000000-000000000000
	github.com/open-telemetry/opentelemetry-collector-contrib/receiver/scraperhelper v0.0.0-00010101000000-000000000000
	github.com/shirou/gopsutil v3.21.7+incompatible
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.33.1-0.20210826200354-479f46434f9a
	go.opentelemetry.io/collector/model v0.33.1-0.20210826200354-479f46434f9a
	go.uber.org/zap v1.19.0
	golang.org/x/sys v0.0.0-20210615035016-665e8c7367d1

)

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/scrapererror => ../scrapererror

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/scraperhelper => ../scraperhelper

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal
