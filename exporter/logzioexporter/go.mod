module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logzioexporter

go 1.16

require (
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/hashicorp/go-hclog v0.16.2
	github.com/jaegertracing/jaeger v1.24.0
	github.com/logzio/jaeger-logzio v1.0.3
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.30.2-0.20210727185145-88b2935343aa
	go.opentelemetry.io/collector/model v0.31.0
	go.uber.org/zap v1.18.1
	google.golang.org/protobuf v1.27.1
)
