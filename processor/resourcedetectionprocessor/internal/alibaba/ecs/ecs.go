// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ecs // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/alibaba/ecs"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/otel/semconv/v1.39.0"
	"go.uber.org/zap"

	ecsprovider "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/alibaba/ecs"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/alibaba/ecs/internal/metadata"
)

const (
	// TypeStr is the detector type id.
	TypeStr = "alibaba_ecs"
)

var _ internal.Detector = (*Detector)(nil)

// Detector queries the Alibaba Cloud ECS metadata service and emits resource attributes.
type Detector struct {
	logger                *zap.Logger
	rb                    *metadata.ResourceBuilder
	metadataProvider      ecsprovider.Provider
	failOnMissingMetadata bool
}

// NewDetector creates an Alibaba Cloud ECS detector.
func NewDetector(set processor.Settings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)

	return &Detector{
		logger:                set.Logger,
		rb:                    metadata.NewResourceBuilder(cfg.ResourceAttributes),
		metadataProvider:      ecsprovider.NewProvider(),
		failOnMissingMetadata: cfg.FailOnMissingMetadata,
	}, nil
}

func (d *Detector) Detect(ctx context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	meta, err := d.metadataProvider.Metadata(ctx)
	if err != nil {
		d.logger.Debug("Alibaba Cloud ECS metadata unavailable", zap.Error(err))
		if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", err
		}
		return pcommon.NewResource(), "", nil
	}

	d.rb.SetCloudProvider(conventions.CloudProviderAlibabaCloud.Value.AsString())
	d.rb.SetCloudPlatform(conventions.CloudPlatformAlibabaCloudECS.Value.AsString())
	d.rb.SetCloudAccountID(meta.OwnerAccountID)
	d.rb.SetCloudRegion(meta.RegionID)
	d.rb.SetCloudAvailabilityZone(meta.ZoneID)
	d.rb.SetHostID(meta.InstanceID)
	d.rb.SetHostName(meta.Hostname)
	d.rb.SetHostImageID(meta.ImageID)
	d.rb.SetHostType(meta.InstanceType)

	return d.rb.Emit(), conventions.SchemaURL, nil
}
