// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package functions // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/azure/functions"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	functionsdetector "go.opentelemetry.io/contrib/detectors/azure/azurefunctions"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/azure/functions/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/sdkbridge"
)

// TypeStr is the detector type string.
const TypeStr = "azurefunctions"

// newResourceDetector is overridden in tests to substitute a fake SDK detector.
var newResourceDetector = func() sdkresource.Detector {
	return functionsdetector.NewResourceDetector()
}

var _ internal.Detector = (*Detector)(nil)

// Detector detects resource attributes when running on Azure Functions. Detection is
// delegated to the upstream SDK detector so that the attributes reported here match the
// ones the collector's own telemetry reports.
type Detector struct {
	detector              sdkresource.Detector
	logger                *zap.Logger
	resourceAttributes    metadata.ResourceAttributesConfig
	failOnMissingMetadata bool
}

// NewDetector creates a new Azure Functions detector.
func NewDetector(p processor.Settings, dcfg internal.DetectorConfig, failOnMissingMetadata bool) (internal.Detector, error) {
	cfg := dcfg.(Config)

	return &Detector{
		detector:              newResourceDetector(),
		logger:                p.Logger,
		resourceAttributes:    cfg.ResourceAttributes,
		failOnMissingMetadata: failOnMissingMetadata,
	}, nil
}

// Detect returns resource attributes when running on Azure Functions.
// Returns an empty resource when not running on Azure Functions, unless
// failOnMissingMetadata is set, in which case it returns an error.
func (d *Detector) Detect(ctx context.Context) (pcommon.Resource, string, error) {
	// Detection runs unfiltered so that an empty result answers "is this process on Azure
	// Functions?"; the configured attributes are applied afterwards.
	res, schemaURL, err := sdkbridge.Detect(ctx, d.detector)
	if err != nil {
		return pcommon.NewResource(), "", err
	}

	if res.Attributes().Len() == 0 {
		d.logger.Debug("Azure Functions detector: not running on Azure Functions")
		if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", errors.New("azure functions metadata unavailable")
		}
		return pcommon.NewResource(), "", nil
	}

	sdkbridge.RemoveDisabledAttributes(res, d.resourceAttributes)
	return res, schemaURL, nil
}
