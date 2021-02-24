module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stackdriverexporter

go 1.14

require (
	contrib.go.opencensus.io/exporter/stackdriver v0.13.5
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v0.16.0
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/stretchr/testify v1.7.0
	go.opencensus.io v0.22.6
	go.opentelemetry.io/collector v0.21.0
	go.opentelemetry.io/otel v0.17.0
	go.opentelemetry.io/otel/sdk v0.16.0
	go.opentelemetry.io/otel/trace v0.17.0
	go.uber.org/zap v1.16.0
	google.golang.org/api v0.40.0
	google.golang.org/genproto v0.0.0-20201214200347-8c77b98c765d
	google.golang.org/grpc v1.35.0
	google.golang.org/grpc/examples v0.0.0-20200728194956-1c32b02682df // indirect
	google.golang.org/protobuf v1.25.0
)
