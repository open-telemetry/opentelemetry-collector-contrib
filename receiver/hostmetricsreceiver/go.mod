module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/hostmetricsreceiver

go 1.16

require (
	github.com/leoluk/perflib_exporter v0.1.0
	github.com/shirou/gopsutil v3.21.7+incompatible
	github.com/spf13/cast v1.4.1
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.32.0
	go.opentelemetry.io/collector/model v0.32.0
	go.uber.org/zap v1.19.0
	golang.org/x/sys v0.0.0-20210816074244-15123e1e1f71
)
