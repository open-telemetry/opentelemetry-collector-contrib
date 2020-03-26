module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stackdriverexporter

go 1.14

require (
	// TODO: pin a released version
	contrib.go.opencensus.io/exporter/stackdriver v0.13.0
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/golang/protobuf v1.3.2
	github.com/open-telemetry/opentelemetry-collector v0.2.8
	github.com/stretchr/testify v1.4.0
	go.opencensus.io v0.22.3
	go.uber.org/zap v1.10.0
	google.golang.org/api v0.10.0
	google.golang.org/genproto v0.0.0-20190911173649-1774047e7e51
	google.golang.org/grpc v1.23.1
)
