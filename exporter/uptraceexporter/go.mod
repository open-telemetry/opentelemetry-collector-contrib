module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/uptraceexporter

go 1.16

require (
	github.com/klauspost/compress v1.13.1
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/stretchr/testify v1.7.0
	github.com/uptrace/uptrace-go v0.21.1
	github.com/vmihailenco/msgpack/v5 v5.3.4
	go.opentelemetry.io/collector v0.29.1-0.20210702000714-32c2d0f13167
	go.opentelemetry.io/collector/model v0.0.0-00010101000000-000000000000
	go.opentelemetry.io/otel v1.0.0-RC1
	go.uber.org/zap v1.18.1
	gopkg.in/square/go-jose.v2 v2.5.1 // indirect
)

replace go.opentelemetry.io/collector/model => go.opentelemetry.io/collector/model v0.0.0-20210702000714-32c2d0f13167
