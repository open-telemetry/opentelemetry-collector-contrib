// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/telemetry"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
)

func IsMetricStatCountSpansSampledEnabled() bool {
	return metadata.ProcessorTailsamplingprocessorMetricstatcountspanssampledFeatureGate.IsEnabled()
}

func IsRecordPolicyEnabled() bool {
	return metadata.ProcessorTailsamplingprocessorRecordpolicyFeatureGate.IsEnabled()
}
