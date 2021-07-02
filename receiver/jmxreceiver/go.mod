module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jmxreceiver

go 1.16

require (
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/shirou/gopsutil v3.21.5+incompatible
	github.com/stretchr/testify v1.7.0
	github.com/testcontainers/testcontainers-go v0.11.1
	go.opentelemetry.io/collector v0.29.1-0.20210702000714-32c2d0f13167
	go.opentelemetry.io/collector/model v0.0.0-00010101000000-000000000000
	go.uber.org/atomic v1.8.0
	go.uber.org/zap v1.18.1
)

replace go.opentelemetry.io/collector/model => go.opentelemetry.io/collector/model v0.0.0-20210702000714-32c2d0f13167
