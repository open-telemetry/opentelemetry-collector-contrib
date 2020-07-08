module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stackdriverexporter

go 1.14

require (
	contrib.go.opencensus.io/exporter/stackdriver v0.13.1
	contrib.go.opencensus.io/resource v0.1.2 // indirect
	github.com/bmizerany/perks v0.0.0-20141205001514-d9a9656a3a4b // indirect
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/golang/protobuf v1.3.5
	github.com/prashantv/protectmem v0.0.0-20171002184600-e20412882b3a // indirect
	github.com/shirou/gopsutil v2.20.4+incompatible // indirect
	github.com/streadway/quantile v0.0.0-20150917103942-b0c588724d25 // indirect
	github.com/stretchr/testify v1.6.1
	github.com/uber/tchannel-go v1.16.0 // indirect
	go.opencensus.io v0.22.3
	go.opentelemetry.io/collector v0.5.1-0.20200708032135-c966e140fd4f
	go.uber.org/zap v1.13.0
	google.golang.org/api v0.13.0
	google.golang.org/genproto v0.0.0-20200513103714-09dca8ec2884
	google.golang.org/grpc v1.29.1
)
