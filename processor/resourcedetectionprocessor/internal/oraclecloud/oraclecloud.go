// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package oraclecloud // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/oraclecloud"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/otel/semconv/v1.30.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/oraclecloud"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/oraclecloud/internal/metadata"
)

const (
	// TypeStr is type of detector.
	TypeStr = "oraclecloud"
)

var _ internal.Detector = (*Detector)(nil)

// Detector is an Oracle Cloud metadata detector
type Detector struct {
	provider oraclecloud.Provider
	logger   *zap.Logger
	rb       *metadata.ResourceBuilder
}

// NewDetector creates a new Oracle Cloud metadata detector
func NewDetector(p processor.Settings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)

	return &Detector{
		provider: oraclecloud.NewProvider(),
		logger:   p.Logger,
		rb:       metadata.NewResourceBuilder(cfg.ResourceAttributes),
	}, nil
}

// Detect detects system metadata and returns a resource with the available ones
func (d *Detector) Detect(ctx context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	compute, err := d.provider.Metadata(ctx)
	if err != nil {
		d.logger.Debug("Oracle Cloud detector metadata retrieval failed!", zap.Error(err))
		// return an empty Resource and no error
		return pcommon.NewResource(), "", nil
	}

	d.rb.SetCloudProvider(conventions.CloudProviderOracleCloud.Value.AsString())
	d.rb.SetCloudPlatform(conventions.CloudPlatformOracleCloudOke.Value.AsString())

	d.rb.SetCloudRegion(compute.RegionID)
	d.rb.SetCloudAvailabilityZone(compute.AvailabilityDomain)
	d.rb.SetHostID(compute.HostID)
	d.rb.SetHostName(compute.HostDisplayName)
	d.rb.SetHostType(compute.HostType)

	d.rb.SetK8sClusterName(compute.Metadata.OKEClusterDisplayName)

	res := d.rb.Emit()

	return res, conventions.SchemaURL, nil
}
