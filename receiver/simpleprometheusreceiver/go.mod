module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/simpleprometheusreceiver

go 1.14

require (
	github.com/ory/x v0.0.109 // indirect
	github.com/prometheus/common v0.10.0
	github.com/prometheus/prometheus v1.8.2-0.20190924101040-52e0504f83ea
	github.com/shirou/gopsutil v2.20.4+incompatible // indirect
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.5.1-0.20200723232356-d4053cc823a0
	go.uber.org/zap v1.15.0
	k8s.io/client-go v0.0.0-20190620085101-78d2af792bab
)
