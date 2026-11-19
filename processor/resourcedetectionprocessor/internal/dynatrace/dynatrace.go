// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

// Package dynatrace provides a detector that loads resource information from
// the dt_host_metadata.properties file which is located in
// the /var/lib/dynatrace/enrichment (on *nix systems) and %ProgramData%\dynatrace\enrichment
// (on Windows) directories.

//go:build (!remove_all_resourcedetection_detectors || enable_detector_dynatrace) && !omit_detector_dynatrace

package dynatrace // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/dynatrace"
import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	dynatracedetector "go.opentelemetry.io/contrib/detectors/dynatrace"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/sdkbridge"
)

// newResourceDetector is overridden in tests to substitute a fake SDK detector.
var newResourceDetector = func() sdkresource.Detector {
	return dynatracedetector.NewResourceDetector()
}

// Ensure Detector implements internal.Detector.
var _ internal.Detector = (*Detector)(nil)

// Detector reads the Dynatrace OneAgent host enrichment file. Detection is
// delegated to the upstream SDK detector.
type Detector struct {
	detector sdkresource.Detector
}

// NewDetector creates a new Dynatrace detector.
func NewDetector(processor.Settings, internal.DetectorConfig, bool) (internal.Detector, error) {
	return &Detector{detector: newResourceDetector()}, nil
}

// Detect returns the attributes declared in the enrichment file. A missing file
// is not an error, as it means the host is not monitored by the OneAgent. A file
// that cannot be read is always an error, regardless of fail_on_missing_metadata.
func (d *Detector) Detect(ctx context.Context) (pcommon.Resource, string, error) {
	return sdkbridge.Detect(ctx, d.detector)
}
