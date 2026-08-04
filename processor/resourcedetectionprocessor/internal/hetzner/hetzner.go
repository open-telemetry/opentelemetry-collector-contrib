// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hetzner // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/hetzner"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	hetznerdetector "go.opentelemetry.io/contrib/detectors/hetzner"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	conventions "go.opentelemetry.io/otel/semconv/v1.42.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/hetzner/internal/metadata"
)

const (
	// TypeStr is type of detector.
	TypeStr = "hetzner"
)

var _ internal.Detector = (*Detector)(nil)

// newResourceDetector is overridden in tests to substitute a fake SDK detector.
var newResourceDetector = func() sdkresource.Detector {
	return hetznerdetector.NewResourceDetector()
}

// Detector is a Hetzner metadata detector. Detection is delegated to the
// upstream SDK detector so that the attributes reported here match the ones the
// collector's own telemetry reports.
type Detector struct {
	detector              sdkresource.Detector
	logger                *zap.Logger
	rb                    *metadata.ResourceBuilder
	failOnMissingMetadata bool
}

// NewDetector creates a new Hetzner metadata detector.
func NewDetector(p processor.Settings, dcfg internal.DetectorConfig, failOnMissingMetadata bool) (internal.Detector, error) {
	cfg := dcfg.(Config)

	return &Detector{
		detector:              newResourceDetector(),
		logger:                p.Logger,
		rb:                    metadata.NewResourceBuilder(cfg.ResourceAttributes),
		failOnMissingMetadata: failOnMissingMetadata,
	}, nil
}

// Detect detects system metadata and returns a resource with the available ones.
func (d *Detector) Detect(ctx context.Context) (pcommon.Resource, string, error) {
	res, err := d.detector.Detect(ctx)
	if err != nil {
		if !errors.Is(err, sdkresource.ErrPartialResource) {
			return pcommon.NewResource(), "", err
		}

		d.logger.Debug("Hetzner detector: some metadata could not be retrieved", zap.Error(err))
		if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", err
		}
	}

	// The SDK detector returns an empty resource when not running on a Hetzner
	// Cloud server.
	if res == nil || len(res.Attributes()) == 0 {
		d.logger.Debug("Hetzner detector: not running on a Hetzner Cloud server")
		return pcommon.NewResource(), "", nil
	}

	for _, attr := range res.Attributes() {
		val := attr.Value.AsString()
		switch attr.Key {
		case conventions.CloudProviderKey:
			d.rb.SetCloudProvider(val)
		case conventions.CloudPlatformKey:
			d.rb.SetCloudPlatform(val)
		case conventions.CloudRegionKey:
			d.rb.SetCloudRegion(val)
		case conventions.CloudAvailabilityZoneKey:
			d.rb.SetCloudAvailabilityZone(val)
		case conventions.HostIDKey:
			d.rb.SetHostID(val)
		case conventions.HostNameKey:
			d.rb.SetHostName(val)
		}
	}

	return d.rb.Emit(), res.SchemaURL(), nil
}
