// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package scaleway // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/scaleway"

import (
	"context"
	"strings"

	instance "github.com/scaleway/scaleway-sdk-go/api/instance/v1"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/otel/semconv/v1.39.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/scaleway/internal/metadata"
)

const (
	// TypeStr is type of detector.
	TypeStr = "scaleway"
)

var _ internal.Detector = (*Detector)(nil)

// newScalewayClient is overridden in tests to point the client at a fake server.
var newScalewayClient = instance.NewMetadataAPI

// Detector is a Scaleway metadata detector.
type Detector struct {
	client *instance.MetadataAPI
	logger *zap.Logger
	rb     *metadata.ResourceBuilder
}

// NewDetector creates a new Scaleway metadata detector.
func NewDetector(p processor.Settings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)

	cli := newScalewayClient()

	return &Detector{
		client: cli,
		logger: p.Logger,
		rb:     metadata.NewResourceBuilder(cfg.ResourceAttributes),
	}, nil
}

// Detect detects system metadata and returns a resource with the available ones.
func (d *Detector) Detect(_ context.Context) (pcommon.Resource, string, error) {
	md, err := d.client.GetMetadata()
	if err != nil || md == nil {
		d.logger.Debug("Scaleway detector: not running on Scaleway or metadata unavailable", zap.Error(err))
		return pcommon.NewResource(), "", nil
	}

	d.rb.SetCloudAccountID(md.Organization)
	d.rb.SetCloudAvailabilityZone(md.Location.ZoneID)
	// Cloud provider and platform values will be "scaleway_cloud" and "scaleway_cloud_platform" from conventions when it's merged.
	// d.rb.SetCloudProvider(conventions.CloudProviderScalewayCloud.Value.AsString())
	// d.rb.SetCloudPlatform(conventions.CloudPlatformScalewayCloud.Value.AsString())
	d.rb.SetCloudProvider("scaleway_cloud")
	d.rb.SetCloudPlatform("scaleway_cloud_compute")
	if region := zoneToRegion(md.Location.ZoneID); region != "" {
		d.rb.SetCloudRegion(region)
	}
	d.rb.SetHostID(md.ID)
	d.rb.SetHostImageID(md.Image.ID)
	d.rb.SetHostImageName(md.Image.Name)
	d.rb.SetHostName(md.Name)
	d.rb.SetHostType(md.CommercialType)

	return d.rb.Emit(), conventions.SchemaURL, nil
}

// zoneToRegion extracts the region name from a Scaleway zone like "fr-par-1" -> "fr-par".
func zoneToRegion(zone string) string {
	if i := strings.LastIndex(zone, "-"); i > 0 {
		return zone[:i]
	}
	return ""
}
