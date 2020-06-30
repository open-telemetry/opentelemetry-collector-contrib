module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver

go 1.14

require (
	github.com/prometheus/common v0.9.1
	github.com/prometheus/prometheus v1.8.2-0.20190924101040-52e0504f83ea
	github.com/stretchr/testify v1.5.1
	go.opentelemetry.io/collector v0.4.1-0.20200629224201-e7a7690e21fc
	go.uber.org/zap v1.14.1
	k8s.io/client-go v0.0.0-20190620085101-78d2af792bab
)
