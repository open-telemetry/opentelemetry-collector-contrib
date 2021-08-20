module github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/opencensus

go 1.16

require (
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/golang/protobuf v1.5.2
	github.com/google/go-cmp v0.5.6
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal v0.0.0-20210820201140-13cd75901f9f
	github.com/stretchr/testify v1.7.0
	go.opencensus.io v0.23.0
	go.opentelemetry.io/collector v0.33.1-0.20210820002854-d3000232f8f6
	go.opentelemetry.io/collector/model v0.33.1-0.20210820002854-d3000232f8f6
	google.golang.org/protobuf v1.27.1
)
