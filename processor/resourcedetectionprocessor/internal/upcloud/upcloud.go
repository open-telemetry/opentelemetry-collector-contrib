// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package upcloud // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/upcloud"

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/otel/semconv/v1.39.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/upcloud"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/upcloud/internal/metadata"
)

const (
	// TypeStr is type of detector.
	TypeStr = "upcloud"
)

// newUpcloudProvider is overridden in tests to point the provider at a fake server.
var newUpcloudProvider = upcloud.NewProvider

// Ensure Detector implements internal.Detector.
var _ internal.Detector = (*Detector)(nil)

// Detector is a Upcloud metadata detector.
type Detector struct {
	provider              upcloud.Provider
	logger                *zap.Logger
	rb                    *metadata.ResourceBuilder
	failOnMissingMetadata bool
}

// NewDetector creates a new Upcloud metadata detector.
func NewDetector(p processor.Settings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)

	return &Detector{
		provider:              newUpcloudProvider(),
		logger:                p.Logger,
		rb:                    metadata.NewResourceBuilder(cfg.ResourceAttributes),
		failOnMissingMetadata: cfg.FailOnMissingMetadata,
	}, nil
}

// Detect queries the Upcloud metadata service and returns a populated resource.
func (d *Detector) Detect(ctx context.Context) (pcommon.Resource, string, error) {
	md, err := d.provider.Metadata(ctx)
	if err != nil {
		d.logger.Debug("Upcloud metadata unavailable", zap.Error(err))
		if d.failOnMissingMetadata {
			return pcommon.NewResource(), "", err
		}
		return pcommon.NewResource(), "", nil
	}

	d.rb.SetCloudProvider(md.CloudName)
	d.rb.SetCloudRegion(md.Region)
	d.rb.SetHostID(md.InstanceID)
	d.rb.SetHostName(md.Hostname)

	return d.rb.Emit(), conventions.SchemaURL, nil
}
