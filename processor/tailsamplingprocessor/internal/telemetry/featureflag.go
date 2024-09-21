// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/telemetry"

import "go.opentelemetry.io/collector/featuregate"

var metricStatCountSpansSampledFeatureGate = featuregate.GlobalRegistry().MustRegister(
	"processor.tailsamplingprocessor.metricstatcountspanssampled",
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, a new metric stat_count_spans_sampled will be available in the tail sampling processor. Differently from stat_count_traces_sampled, this metric will count the number of spans sampled or not per sampling policy, where the original counts traces."),
)

func IsMetricStatCountSpansSampledEnabled() bool {
	return metricStatCountSpansSampledFeatureGate.IsEnabled()
}
