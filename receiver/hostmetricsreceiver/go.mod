module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver

go 1.17

require (
	github.com/leoluk/perflib_exporter v0.1.0
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.47.0
	github.com/shirou/gopsutil/v3 v3.22.2
	github.com/stretchr/testify v1.7.1
	go.opentelemetry.io/collector v0.47.1-0.20220330050215-0f07b0bd64d1
	go.opentelemetry.io/collector/model v0.47.1-0.20220330050215-0f07b0bd64d1
	go.uber.org/zap v1.21.0
	golang.org/x/sys v0.0.0-20220114195835-da31bd327af9

)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal => ../../internal/coreinternal
