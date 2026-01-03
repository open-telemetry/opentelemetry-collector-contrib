module github.com/open-telemetry/opentelemetry-collector-contrib

// NOTE:
// This go.mod is NOT used to build any official binary.
// To see the builder manifests used for official binaries,
// check https://github.com/open-telemetry/opentelemetry-collector-releases
//
// For the OpenTelemetry Collector Contrib distribution specifically, see
// https://github.com/open-telemetry/opentelemetry-collector-releases/tree/main/distributions/otelcol-contrib

go 1.24.0

retract (
	v0.76.2
	v0.76.1
	v0.65.0
	v0.37.0 // Contains dependencies on v0.36.0 components, which should have been updated to v0.37.0.
)

replace github.com/open-telemetry/opentelemetry-collector-contrib/receiver/k8sobjectsreceiver => ./receiver/k8sobjectsreceiver

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil => ./pkg/pdatautil

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/xk8stest => ./pkg/xk8stest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest => ./pkg/pdatatest

replace github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden => ./pkg/golden

replace github.com/open-telemetry/opentelemetry-collector-contrib/internal/k8sconfig => ./internal/k8sconfig

replace github.com/open-telemetry/opentelemetry-collector-contrib/extension/k8sleaderelector => ./extension/k8sleaderelector
