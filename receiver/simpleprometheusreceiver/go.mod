module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver

go 1.14

require (
	github.com/open-telemetry/opentelemetry-collector v0.3.1-0.20200413233902-380a0e75c518
	github.com/prometheus/common v0.9.1
	github.com/prometheus/prometheus v1.8.2-0.20190924101040-52e0504f83ea
	github.com/stretchr/testify v1.4.0
	go.uber.org/zap v1.14.1
	k8s.io/client-go v0.0.0-20190620085101-78d2af792bab
)
