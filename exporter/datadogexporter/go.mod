module github.com/DataDog/opentelemetry-collector-contrib/exporter/datadogexporter

go 1.15

require (
	github.com/DataDog/datadog-go v4.0.0+incompatible
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.9.0
	go.uber.org/zap v1.15.0
	google.golang.org/protobuf v1.25.0
)
