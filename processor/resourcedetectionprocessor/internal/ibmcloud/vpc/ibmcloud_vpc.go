// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vpc // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/ibmcloud/vpc"

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	vpcdetector "go.opentelemetry.io/contrib/detectors/ibmcloud/vpc"
	sdkresource "go.opentelemetry.io/otel/sdk/resource"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/ibmcloud/vpc/internal/metadata"
)

const (
	// TypeStr is the detector type string.
	TypeStr = "ibmcloud_vpc"
)

// newResourceDetector is overridden in tests to substitute a fake SDK detector.
var newResourceDetector = func(protocol string) sdkresource.Detector {
	return vpcdetector.NewResourceDetector(vpcdetector.WithProtocol(protocol))
}

var _ internal.Detector = (*Detector)(nil)

// Detector queries the IBM Cloud VPC Instance Metadata Service and emits resource attributes.
// Detection is delegated to the upstream SDK detector so that the attributes reported here match
// the ones the collector's own telemetry reports.
type Detector struct {
	detector              sdkresource.Detector
	logger                *zap.Logger
	rb                    *metadata.ResourceBuilder
	failOnMissingMetadata bool
}

// NewDetector creates an IBM Cloud VPC detector.
func NewDetector(p processor.Settings, dcfg internal.DetectorConfig, failOnMissingMetadata bool) (internal.Detector, error) {
	cfg := dcfg.(Config)

	switch cfg.Protocol {
	case "http", "https":
	default:
		return nil, fmt.Errorf("invalid protocol %q: must be \"http\" or \"https\"", cfg.Protocol)
	}

	return &Detector{
		detector:              newResourceDetector(cfg.Protocol),
		logger:                p.Logger,
		rb:                    metadata.NewResourceBuilder(cfg.ResourceAttributes),
		failOnMissingMetadata: failOnMissingMetadata,
	}, nil
}

// Detect detects IBM Cloud VPC instance metadata and returns a resource with the available attributes.
func (d *Detector) Detect(ctx context.Context) (pcommon.Resource, string, error) {
	res, err := d.detector.Detect(ctx)
	if err != nil {
		d.logger.Debug("IBM Cloud VPC metadata not available", zap.Error(err))
		// A partial result still came from a reachable metadata service, so keep what it did
		// return. fail_on_missing_metadata covers an unusable metadata service, not individual
		// fields absent from its response.
		if !errors.Is(err, sdkresource.ErrPartialResource) {
			if d.failOnMissingMetadata {
				return pcommon.NewResource(), "", fmt.Errorf("ibmcloud vpc metadata unavailable: %w", err)
			}
			return pcommon.NewResource(), "", nil
		}
	}

	// The SDK detector reports an empty resource and no error both when the metadata service is
	// unreachable and when not running on an IBM Cloud VPC instance.
	if res.Len() == 0 {
		d.logger.Debug("IBM Cloud VPC detector: metadata unavailable or not running on a VPC instance")
		if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", errors.New("ibmcloud vpc metadata unavailable")
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
		case conventions.CloudAvailabilityZoneKey:
			d.rb.SetCloudAvailabilityZone(val)
		case conventions.CloudAccountIDKey:
			d.rb.SetCloudAccountID(val)
		case conventions.CloudResourceIDKey:
			d.rb.SetCloudResourceID(val)
		case conventions.HostIDKey:
			d.rb.SetHostID(val)
		case conventions.HostImageIDKey:
			d.rb.SetHostImageID(val)
		case conventions.HostImageNameKey:
			d.rb.SetHostImageName(val)
		case conventions.HostNameKey:
			d.rb.SetHostName(val)
		case conventions.HostTypeKey:
			d.rb.SetHostType(val)
		}
	}

	return d.rb.Emit(), conventions.SchemaURL, nil
}
