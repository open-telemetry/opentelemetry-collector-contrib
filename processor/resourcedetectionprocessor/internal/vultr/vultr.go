// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package vultr // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/vultr"

import (
	"context"
	"strings"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/otel/semconv/v1.39.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/vultr"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/vultr/internal/metadata"
)

const (
	// TypeStr is type of detector.
	TypeStr = "vultr"
)

// newVultrProvider is overridden in tests to point the provider at a fake server.
var newVultrProvider = vultr.NewProvider

// Ensure Detector implements internal.Detector.
var _ internal.Detector = (*Detector)(nil)

// Detector is a Vultr metadata detector.
type Detector struct {
	provider              vultr.Provider
	logger                *zap.Logger
	rb                    *metadata.ResourceBuilder
	failOnMissingMetadata bool
}

// NewDetector creates a new Vultr metadata detector.
func NewDetector(p processor.Settings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)

	return &Detector{
		provider:              newVultrProvider(),
		logger:                p.Logger,
		rb:                    metadata.NewResourceBuilder(cfg.ResourceAttributes),
		failOnMissingMetadata: cfg.FailOnMissingMetadata,
	}, nil
}

// Detect queries the Vultr metadata service and returns a populated resource.
func (d *Detector) Detect(ctx context.Context) (pcommon.Resource, string, error) {
	md, err := d.provider.Metadata(ctx)
	if err != nil {
		d.logger.Debug("Vultr metadata unavailable", zap.Error(err))
		if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", err
		}
		return pcommon.NewResource(), "", nil
	}

	// Prefer the v2 UUID if present; fall back to the legacy instance ID.
	hostID := md.InstanceV2ID
	if hostID == "" {
		hostID = md.InstanceID
	}

	d.rb.SetCloudProvider(conventions.CloudProviderVultr.Value.AsString())
	d.rb.SetCloudPlatform(conventions.CloudPlatformVultrCloudCompute.Value.AsString())
	d.rb.SetCloudRegion(strings.ToLower(md.Region.RegionCode))
	d.rb.SetHostID(hostID)
	d.rb.SetHostName(md.Hostname)

	return d.rb.Emit(), conventions.SchemaURL, nil
}
