module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azuremonitorexporter

go 1.14

require (
	code.cloudfoundry.org/clock v0.0.0-20180518195852-02e53af36e6c // indirect
	github.com/Microsoft/ApplicationInsights-Go v0.4.2
	github.com/aws/aws-sdk-go v1.23.20 // indirect
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/golang/protobuf v1.3.5
	github.com/stretchr/testify v1.5.1
	github.com/tedsuo/ifrit v0.0.0-20191009134036-9a97d0632f00 // indirect
	go.opentelemetry.io/collector v0.4.1-0.20200629224201-e7a7690e21fc
	go.uber.org/zap v1.13.0
	google.golang.org/appengine v1.6.2 // indirect
	google.golang.org/grpc v1.29.1
)
