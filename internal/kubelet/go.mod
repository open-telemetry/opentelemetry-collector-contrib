module github.com/open-telemetry/opentelemetry-collector-contrib/internal/kubelet

go 1.16

require (
	github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig v0.0.0-00010101000000-000000000000
	github.com/stretchr/testify v1.7.0
	go.opentelemetry.io/collector v0.29.1-0.20210712222151-aa60edff162c
	go.uber.org/zap v1.18.1
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig => ../../internal/k8sconfig

replace go.opentelemetry.io/collector/model => go.opentelemetry.io/collector/model v0.0.0-20210712222151-aa60edff162c
