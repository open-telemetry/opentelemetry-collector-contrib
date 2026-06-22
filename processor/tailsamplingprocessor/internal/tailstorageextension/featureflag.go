// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailstorageextension // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/tailstorageextension"

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
)

var FeatureGateID = metadata.ProcessorTailsamplingprocessorTailstorageextensionFeatureGate.ID()

func IsFeatureGateEnabled() bool {
	return metadata.ProcessorTailsamplingprocessorTailstorageextensionFeatureGate.IsEnabled()
}
