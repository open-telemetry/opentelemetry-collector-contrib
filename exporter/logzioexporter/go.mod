module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/logzioexporter

go 1.16

require (
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/hashicorp/go-hclog v0.16.1
	github.com/jaegertracing/jaeger v1.23.0
	github.com/logzio/jaeger-logzio v1.0.2
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/onsi/ginkgo v1.14.1 // indirect
	github.com/onsi/gomega v1.10.2 // indirect
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.29.1-0.20210702000714-32c2d0f13167
	go.opentelemetry.io/collector/model v0.0.0-00010101000000-000000000000
	go.uber.org/zap v1.18.1
	google.golang.org/protobuf v1.27.1
	gopkg.in/square/go-jose.v2 v2.5.1 // indirect
)

replace go.opentelemetry.io/collector/model => go.opentelemetry.io/collector/model v0.0.0-20210702000714-32c2d0f13167
