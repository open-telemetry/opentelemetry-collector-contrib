module github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpertrace

go 1.16

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector/model v0.0.0-00010101000000-000000000000
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/batchpersignal => ../batchpersignal

replace go.opentelemetry.io/collector/model => go.opentelemetry.io/collector/model v0.0.0-20210630154722-7d0a0398174e
