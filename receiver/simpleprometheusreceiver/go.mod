module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver

go 1.14

require (
	github.com/prometheus/common v0.14.0
	github.com/prometheus/prometheus v1.8.2-0.20200827201422-1195cc24e3c8
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.13.0
	go.uber.org/zap v1.16.0
	google.golang.org/grpc/examples v0.0.0-20200728194956-1c32b02682df // indirect
	k8s.io/client-go v0.19.2
)
