module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logzioexporter

go 1.14
require (
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/hashicorp/go-hclog v0.14.0
	github.com/jaegertracing/jaeger v1.19.2
	github.com/logzio/jaeger-logzio v0.0.0-20200826115713-de8961d427e3
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.11.1-0.20201001213035-035aa5cf6c92
	go.uber.org/zap v1.16.0
	google.golang.org/protobuf v1.25.0
)
