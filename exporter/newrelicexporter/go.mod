module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/newrelicexporter

go 1.16

require (
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/golang/protobuf v1.5.2
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/newrelic/newrelic-telemetry-sdk-go v0.8.1
	github.com/stretchr/testify v1.7.0
	go.opencensus.io v0.23.0
	go.opentelemetry.io/collector v0.33.1-0.20210820002854-d3000232f8f6
	go.opentelemetry.io/collector/model v0.33.1-0.20210820002854-d3000232f8f6
	go.uber.org/zap v1.19.0
	google.golang.org/genproto v0.0.0-20210604141403-392c879c8b08
	google.golang.org/grpc v1.40.0
	google.golang.org/protobuf v1.27.1
)
