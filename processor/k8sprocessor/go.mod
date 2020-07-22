module github.com/open-telemetry/opentelemetry-collector-contrib/processor/k8sprocessor

go 1.14

require (
	github.com/census-instrumentation/opencensus-proto v0.2.1
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/common v0.4.0
	github.com/shirou/gopsutil v2.20.4+incompatible // indirect
	github.com/stretchr/testify v1.6.1
	go.opencensus.io v0.22.4
	go.opentelemetry.io/collector v0.5.1-0.20200722180048-c0b3cf61a63a
	go.uber.org/zap v1.15.0
	k8s.io/api v0.17.0
	k8s.io/apimachinery v0.17.0
	k8s.io/client-go v0.0.0-20190620085101-78d2af792bab
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/common => ../../internal/common
