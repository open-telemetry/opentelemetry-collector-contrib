module github.com/open-telemetry/opentelemetry-collector-contrib/receiver/nginxreceiver

go 1.14

require (
	github.com/containerd/containerd v1.3.6 // indirect
	github.com/nginxinc/nginx-prometheus-exporter v0.8.0
	github.com/stretchr/testify v1.6.1
	go.opentelemetry.io/collector v0.13.1-0.20201027215027-6ae66159741d
	go.uber.org/zap v1.16.0
	google.golang.org/api v0.32.0 // indirect
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common
