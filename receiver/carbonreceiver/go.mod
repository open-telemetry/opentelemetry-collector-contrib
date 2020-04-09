module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver

go 1.14

require (
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/golang/protobuf v1.3.5
	github.com/open-telemetry/opentelemetry-collector v0.3.1-0.20200408203355-0e1b2e323d39
	github.com/spf13/viper v1.6.2
	github.com/stretchr/testify v1.4.0
	go.opencensus.io v0.22.3
	go.uber.org/zap v1.13.0
	k8s.io/client-go v12.0.0+incompatible // indirect
)
