// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourcedetectionprocessor

import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"

func availableDetectorFactories() map[internal.DetectorType]internal.DetectorFactory {
	available := detectorFactories()
	for detectorType, factory := range available {
		if factory == nil {
			delete(available, detectorType)
		}
	}
	return available
}
