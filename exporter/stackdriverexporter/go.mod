module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stackdriverexporter

go 1.14

require (
	contrib.go.opencensus.io/exporter/stackdriver v0.13.4
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v0.10.1-0.20200828070728-ba72e89a4087
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/opentracing/opentracing-go v1.2.0 // indirect
	github.com/stretchr/testify v1.6.1
	go.opencensus.io v0.22.4
	go.opentelemetry.io/collector v0.9.1-0.20200901221426-ec327358d634
	go.opentelemetry.io/otel v0.11.0
	go.opentelemetry.io/otel/sdk v0.11.0
	go.uber.org/zap v1.15.0
	google.golang.org/api v0.30.0
	google.golang.org/genproto v0.0.0-20200804131852-c06518451d9c
	google.golang.org/grpc v1.31.1
	google.golang.org/grpc/examples v0.0.0-20200728194956-1c32b02682df // indirect
	google.golang.org/protobuf v1.25.0
)
