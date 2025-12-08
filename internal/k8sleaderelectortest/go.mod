module github.com/open-telemetry/opentelemetry-collector-contrib/extension/k8sleaderelector/k8sleaderelectortest

go 1.24.0

require (
	go.opentelemetry.io/collector/component v1.47.0
	go.opentelemetry.io/collector/pipeline v1.47.0

	github.com/open-telemetry/opentelemetry-collector-contrib/extension/k8sleaderelector v0.141.0
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/k8sleaderelector => ../

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig => ../../../internal/k8sconfig
