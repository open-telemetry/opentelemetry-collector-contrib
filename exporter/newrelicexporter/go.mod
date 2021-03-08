module github.com/open-telemetry/opentelemetry-collector-contrib/exporter/newrelicexporter

go 1.14

require (
	github.com/census-instrumentation/opencensus-proto v0.3.0
	github.com/golang/protobuf v1.4.3
	github.com/newrelic/newrelic-telemetry-sdk-go v0.5.2
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.21.1-0.20210305192649-7cdbd1caab8b
	go.uber.org/zap v1.16.0
	google.golang.org/genproto v0.0.0-20210302174412-5ede27ff9881
	google.golang.org/grpc v1.36.0
	google.golang.org/grpc/examples v0.0.0-20200728194956-1c32b02682df // indirect
	google.golang.org/protobuf v1.25.0
)
