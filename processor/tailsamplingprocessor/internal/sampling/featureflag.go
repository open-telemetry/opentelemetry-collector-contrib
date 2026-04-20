// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sampling // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/sampling"

import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"

func IsInvertDecisionsDisabled() bool {
	return metadata.ProcessorTailsamplingprocessorDisableinvertdecisionsFeatureGate.IsEnabled()
}
