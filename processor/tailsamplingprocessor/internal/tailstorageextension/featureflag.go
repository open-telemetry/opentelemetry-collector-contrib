// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailstorageextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/tailstorageextension"

import "go.opentelemetry.io/collector/featuregate"

const FeatureGateID = "processor.tailsamplingprocessor.tailstorageextension"

var featureGate = featuregate.GlobalRegistry().MustRegister(
	FeatureGateID,
	featuregate.StageAlpha,
	featuregate.WithRegisterDescription("When enabled, allows configuring tail_storage to use a tail storage extension implementation."),
)

func IsFeatureGateEnabled() bool {
	return featureGate.IsEnabled()
}
