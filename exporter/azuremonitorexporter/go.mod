module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter

go 1.16

require (
	code.cloudfoundry.org/clock v1.0.0 // indirect
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/microsoft/ApplicationInsights-Go v0.4.4
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.29.1-0.20210702000714-32c2d0f13167
	go.opentelemetry.io/collector/model v0.0.0-00010101000000-000000000000
	go.uber.org/zap v1.18.1
	golang.org/x/net v0.0.0-20210614182718-04defd469f4e
	gopkg.in/square/go-jose.v2 v2.5.1 // indirect
)

replace go.opentelemetry.io/collector/model => go.opentelemetry.io/collector/model v0.0.0-20210702000714-32c2d0f13167
