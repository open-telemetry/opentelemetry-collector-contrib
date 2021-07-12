module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/sentryexporter

go 1.16

require (
	github.com/getsentry/sentry-go v0.11.0
	github.com/google/go-cmp v0.5.6
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.29.1-0.20210708180338-a4354b6e8e39
	go.opentelemetry.io/collector/model v0.0.0-00010101000000-000000000000
	gopkg.in/square/go-jose.v2 v2.5.1 // indirect
)

replace go.opentelemetry.io/collector/model => go.opentelemetry.io/collector/model v0.0.0-20210708180338-a4354b6e8e39
