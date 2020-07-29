module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stackdriverexporter

go 1.14

require (
	contrib.go.opencensus.io/exporter/stackdriver v0.13.1
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/golang/protobuf v1.4.2
	github.com/stretchr/testify v1.6.1
	go.opencensus.io v0.22.4
	go.opentelemetry.io/collector v0.5.1-0.20200728200651-9cbf43e372f0
	go.uber.org/zap v1.15.0
	google.golang.org/api v0.29.0
	google.golang.org/genproto v0.0.0-20200624020401-64a14ca9d1ad
	google.golang.org/grpc v1.30.0
	google.golang.org/grpc/examples v0.0.0-20200728194956-1c32b02682df // indirect
)
