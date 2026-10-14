// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build remove_all_resourcedetection_detectors

package resourcedetectionprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor"

import "go.uber.org/zap"

func logDetectorConfiguration(logger *zap.Logger, compiled, configured []string) {
	logger.Info("resource detection processor detector configuration",
		zap.Strings("compiled_detectors", compiled),
		zap.Strings("configured_detectors", configured))
}
