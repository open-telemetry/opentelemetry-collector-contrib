module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logzioexporter

go 1.16

require (
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/hashicorp/go-hclog v0.16.2
	github.com/jaegertracing/jaeger v1.24.0
	github.com/logzio/jaeger-logzio v1.0.3
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.31.1-0.20210804191544-3cfe4f8c5d3e
	go.opentelemetry.io/collector/model v0.31.1-0.20210804191544-3cfe4f8c5d3e
	go.uber.org/zap v1.18.1
	google.golang.org/protobuf v1.27.1
)
