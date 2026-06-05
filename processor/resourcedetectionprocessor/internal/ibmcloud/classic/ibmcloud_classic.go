// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package classic // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/ibmcloud/classic"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.uber.org/zap"

	classicprovider "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/ibmcloud/classic"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/ibmcloud/classic/internal/metadata"
)

const (
	// TypeStr is the detector type string.
	TypeStr = "ibmcloud_classic"
)

var _ internal.Detector = (*Detector)(nil)

// Detector queries the IBM Cloud Classic (SoftLayer) Resource Metadata Service
// and emits resource attributes.
type Detector struct {
	provider classicprovider.Provider
	logger   *zap.Logger
	rb       *metadata.ResourceBuilder
}

// NewDetector creates an IBM Cloud Classic detector.
func NewDetector(p processor.Settings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)

	return &Detector{
		provider: classicprovider.NewProvider(),
		logger:   p.Logger,
		rb:       metadata.NewResourceBuilder(cfg.ResourceAttributes),
	}, nil
}

// Detect detects IBM Cloud Classic instance metadata and returns a resource with the available attributes.
func (d *Detector) Detect(ctx context.Context) (pcommon.Resource, string, error) {
	meta, err := d.provider.InstanceMetadata(ctx)
	if err != nil {
		d.logger.Debug("IBM Cloud Classic metadata not available", zap.Error(err))
		return pcommon.NewResource(), "", nil
	}

	d.rb.SetCloudProvider(conventions.CloudProviderIBMCloud.Value.AsString())
	// TODO: Use semconv constant once CloudPlatformIBMCloudClassic is added.
	d.rb.SetCloudPlatform("ibm_cloud.classic")
	d.rb.SetCloudAccountID(meta.AccountID)
	d.rb.SetCloudAvailabilityZone(meta.Datacenter)
	d.rb.SetCloudResourceID(meta.GlobalIdentifier)
	d.rb.SetHostID(meta.ID)
	d.rb.SetHostName(meta.Hostname)

	return d.rb.Emit(), conventions.SchemaURL, nil
}
