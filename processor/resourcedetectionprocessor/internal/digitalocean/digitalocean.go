// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build (!remove_all_resourcedetection_detectors || enable_detector_digitalocean) && !omit_detector_digitalocean

package digitalocean // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/digitalocean"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	dodetector "go.opentelemetry.io/contrib/detectors/digitalocean"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/digitalocean/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/sdkbridge"
)

var _ internal.Detector = (*Detector)(nil)

// newResourceDetector is overridden in tests to substitute a fake SDK detector.
var newResourceDetector = func() sdkresource.Detector {
	return dodetector.NewResourceDetector()
}

// Detector is a DigitalOcean metadata detector. Detection is delegated to the upstream
// SDK detector so that the attributes reported here match the ones the collector's own
// telemetry reports.
type Detector struct {
	detector              sdkresource.Detector
	logger                *zap.Logger
	resourceAttributes    metadata.ResourceAttributesConfig
	failOnMissingMetadata bool
}

// NewDetector creates a new DigitalOcean metadata detector.
func NewDetector(p processor.Settings, dcfg internal.DetectorConfig, failOnMissingMetadata bool) (internal.Detector, error) {
	cfg := dcfg.(Config)

	return &Detector{
		detector:              newResourceDetector(),
		logger:                p.Logger,
		resourceAttributes:    cfg.ResourceAttributes,
		failOnMissingMetadata: failOnMissingMetadata,
	}, nil
}

// Detect queries the DigitalOcean metadata service and returns a populated resource.
func (d *Detector) Detect(ctx context.Context) (pcommon.Resource, string, error) {
	// A partial result still came from a reachable metadata service, so the bridge keeps
	// what it did return. fail_on_missing_metadata covers an unusable metadata service,
	// not individual fields absent from its response.
	res, schemaURL, err := sdkbridge.Detect(ctx, d.detector)
	if err != nil {
		d.logger.Debug("DigitalOcean metadata unavailable", zap.Error(err))
		if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", err
		}
		return pcommon.NewResource(), "", nil
	}

	// The SDK detector reports an empty resource and no error when not running on a Droplet.
	if res.Attributes().Len() == 0 {
		d.logger.Debug("DigitalOcean detector: not running on a Droplet")
		if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", errors.New("digitalocean metadata unavailable")
		}
		return pcommon.NewResource(), "", nil
	}

	sdkbridge.RemoveDisabledAttributes(res, d.resourceAttributes)
	return res, schemaURL, nil
}
