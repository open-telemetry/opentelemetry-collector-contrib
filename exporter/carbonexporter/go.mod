module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/carbonexporter

go 1.16

require (
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testutil v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.33.1-0.20210820002854-d3000232f8f6
	go.opentelemetry.io/collector/model v0.33.1-0.20210820002854-d3000232f8f6
	google.golang.org/protobuf v1.27.1
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testutil => ../../internal/coreinternal/testutil
