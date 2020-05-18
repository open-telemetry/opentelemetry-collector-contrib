module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stackdriverexporter

go 1.14

require (
	contrib.go.opencensus.io/exporter/stackdriver v0.13.1
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/golang/protobuf v1.3.5
	github.com/stretchr/testify v1.5.1
	go.opencensus.io v0.22.3
	go.opentelemetry.io/collector v0.3.1-0.20200518164231-3729dac06f74
	go.uber.org/zap v1.10.0
	google.golang.org/api v0.10.0
	google.golang.org/genproto v0.0.0-20200408120641-fbb3ad325eb7
	google.golang.org/grpc v1.29.1
)
