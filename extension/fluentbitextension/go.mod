module github.com/open-telemetry/opentelemetry-collector-contrib/extension/fluentbitextension

go 1.16

require (
	github.com/davecgh/go-spew v1.1.1
	github.com/shirou/gopsutil v3.21.6+incompatible
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.29.1-0.20210701204331-d1fced9688ba
	go.uber.org/zap v1.18.1
	gopkg.in/square/go-jose.v2 v2.5.1 // indirect
)

replace go.opentelemetry.io/collector/model => go.opentelemetry.io/collector/model v0.0.0-20210701204331-d1fced9688ba
