// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build (remove_all_resourcedetection_detectors && !enable_detector_scaleway) || omit_detector_scaleway

package scaleway // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/scaleway"

import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"

// NewDetector is unavailable when this detector is excluded at build time.
var NewDetector internal.DetectorFactory
