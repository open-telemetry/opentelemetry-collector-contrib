// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cvm // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/tencent/cvm"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
	"go.uber.org/zap"

	cvmprovider "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/tencent/cvm"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/tencent/cvm/internal/metadata"
)

const (
	// TypeStr is the detector type id.
	TypeStr = "tencent_cvm"
)

var _ internal.Detector = (*Detector)(nil)

// Detector queries the Tencent Cloud CVM metadata service and emits resource attributes.
type Detector struct {
	logger                *zap.Logger
	rb                    *metadata.ResourceBuilder
	metadataProvider      cvmprovider.Provider
	failOnMissingMetadata bool
}

// NewDetector creates a Tencent Cloud CVM detector.
func NewDetector(set processor.Settings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)

	return &Detector{
		logger:                set.Logger,
		rb:                    metadata.NewResourceBuilder(cfg.ResourceAttributes),
		metadataProvider:      cvmprovider.NewProvider(),
		failOnMissingMetadata: cfg.FailOnMissingMetadata,
	}, nil
}

func (d *Detector) Detect(ctx context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	meta, err := d.metadataProvider.Metadata(ctx)
	if err != nil {
		d.logger.Debug("Tencent Cloud CVM metadata unavailable", zap.Error(err))
		if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", err
		}
		return pcommon.NewResource(), "", nil
	}

	d.rb.SetCloudProvider(conventions.CloudProviderTencentCloud.Value.AsString())
	d.rb.SetCloudPlatform(conventions.CloudPlatformTencentCloudCVM.Value.AsString())
	d.rb.SetCloudAccountID(meta.AppID)
	d.rb.SetCloudRegion(meta.RegionID)
	d.rb.SetCloudAvailabilityZone(meta.ZoneID)
	d.rb.SetHostID(meta.InstanceID)
	d.rb.SetHostName(meta.InstanceName)
	d.rb.SetHostImageID(meta.ImageID)
	d.rb.SetHostType(meta.InstanceType)

	return d.rb.Emit(), conventions.SchemaURL, nil
}
