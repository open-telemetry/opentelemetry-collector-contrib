module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticexporter

go 1.16

require (
	github.com/elastic/go-sysinfo v1.4.0 // indirect
	github.com/elastic/go-windows v1.0.1 // indirect
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/stretchr/testify v1.7.0
	go.elastic.co/apm v1.9.1-0.20201218004853-18a8126106c6
	go.elastic.co/fastjson v1.1.0
	go.opentelemetry.io/collector v0.29.1-0.20210702000714-32c2d0f13167
	go.opentelemetry.io/collector/model v0.0.0-00010101000000-000000000000
	go.uber.org/zap v1.18.1
	gopkg.in/square/go-jose.v2 v2.5.1 // indirect
	howett.net/plist v0.0.0-20201026045517-117a925f2150 // indirect
)

replace go.opentelemetry.io/collector/model => go.opentelemetry.io/collector/model v0.0.0-20210702000714-32c2d0f13167
