module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver

go 1.14

require (
	github.com/Azure/go-autorest/autorest/adal v0.9.0 // indirect
	github.com/prometheus/common v0.11.1
	github.com/prometheus/prometheus v1.8.2-0.20200626085723-c448ada63d83
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.10.0
	go.uber.org/zap v1.16.0
	google.golang.org/grpc/examples v0.0.0-20200728194956-1c32b02682df // indirect
	k8s.io/client-go v0.19.1
)
