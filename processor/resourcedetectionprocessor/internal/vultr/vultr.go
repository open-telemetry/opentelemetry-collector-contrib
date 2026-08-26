// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vultr // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/vultr"

import (
	"context"
	"errors"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	vultrdetector "go.opentelemetry.io/contrib/detectors/vultr"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/vultr/internal/metadata"
)

const (
	// TypeStr is type of detector.
	TypeStr = "vultr"
)

// newResourceDetector is overridden in tests to substitute a fake SDK detector.
var newResourceDetector = func() sdkresource.Detector {
	return vultrdetector.NewResourceDetector()
}

// Ensure Detector implements internal.Detector.
var _ internal.Detector = (*Detector)(nil)

// Detector is a Vultr metadata detector. Detection is delegated to the upstream
// SDK detector so that the attributes reported here match the ones the
// collector's own telemetry reports.
type Detector struct {
	detector              sdkresource.Detector
	logger                *zap.Logger
	rb                    *metadata.ResourceBuilder
	failOnMissingMetadata bool
}

// NewDetector creates a new Vultr metadata detector.
func NewDetector(p processor.Settings, dcfg internal.DetectorConfig, failOnMissingMetadata bool) (internal.Detector, error) {
	cfg := dcfg.(Config)

	return &Detector{
		detector:              newResourceDetector(),
		logger:                p.Logger,
		rb:                    metadata.NewResourceBuilder(cfg.ResourceAttributes),
		failOnMissingMetadata: failOnMissingMetadata || cfg.FailOnMissingMetadata,
	}, nil
}

// Detect queries the Vultr metadata service and returns a populated resource.
func (d *Detector) Detect(ctx context.Context) (pcommon.Resource, string, error) {
	res, err := d.detector.Detect(ctx)
	if err != nil {
		d.logger.Debug("Vultr metadata unavailable", zap.Error(err))
		// A partial result still came from a reachable metadata service, so keep
		// what it did return. fail_on_missing_metadata covers an unusable
		// metadata service, not individual fields absent from its response.
		if !errors.Is(err, sdkresource.ErrPartialResource) {
			if d.failOnMissingMetadata {
				return pcommon.NewResource(), "", err
			}
			return pcommon.NewResource(), "", nil
		}
	}

	// The SDK detector reports an empty resource and no error both when the
	// metadata service is unreachable and when running outside Vultr.
	if res.Len() == 0 {
		d.logger.Debug("Vultr detector: metadata unavailable or not running on a Vultr instance")
		if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", errors.New("vultr metadata unavailable")
		}
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
		case conventions.HostIDKey:
			d.rb.SetHostID(val)
		case conventions.HostNameKey:
			d.rb.SetHostName(val)
		}
	}

	return d.rb.Emit(), conventions.SchemaURL, nil
}
