module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/googlecloudexporter

go 1.16

require (
	contrib.go.opencensus.io/exporter/stackdriver v0.13.8
	github.com/GoogleCloudPlatform/opentelemetry-operations-go/exporter/trace v0.20.1-0.20210712185927-c0aa9a97e391
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/mattn/go-colorable v0.1.7 // indirect
	github.com/stretchr/testify v1.7.0
	go.opencensus.io v0.23.0
	go.opentelemetry.io/collector v0.30.0
	go.opentelemetry.io/collector/model v0.30.0
	go.opentelemetry.io/otel v1.0.0-RC1
	go.opentelemetry.io/otel/sdk v1.0.0-RC1
	go.opentelemetry.io/otel/trace v1.0.0-RC1
	go.uber.org/zap v1.18.1
	google.golang.org/api v0.50.0
	google.golang.org/genproto v0.0.0-20210624195500-8bfb893ecb84
	google.golang.org/grpc v1.39.0
	google.golang.org/protobuf v1.27.1
	gopkg.in/square/go-jose.v2 v2.5.1 // indirect
)
