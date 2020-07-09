module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stackdriverexporter

go 1.14

replace github.com/open-telemetry/opentelemetry-collector v0.2.6 => github.com/pmm-sumo/opentelemetry-collector v0.2.7-0.20200311163619-eb9e7f3949fb

require (
	contrib.go.opencensus.io/exporter/stackdriver v0.13.1
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/golang/protobuf v1.3.5
	github.com/shirou/gopsutil v2.20.4+incompatible // indirect
	github.com/stretchr/testify v1.5.1
	go.opencensus.io v0.22.3
	go.opentelemetry.io/collector v0.5.0
	go.uber.org/zap v1.13.0
	google.golang.org/api v0.10.0
	google.golang.org/genproto v0.0.0-20200408120641-fbb3ad325eb7
	google.golang.org/grpc v1.29.1
)
