// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hetzner // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/hetzner"

import (
	"context"
	"fmt"

	hcloudmeta "github.com/hetznercloud/hcloud-go/v2/hcloud/metadata"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/otel/semconv/v1.6.1"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/hetzner/internal/metadata"
)

const (
	// TypeStr is type of detector.
	TypeStr = "hetzner"
)

var _ internal.Detector = (*Detector)(nil)

// newHcloudClient is overridden in tests to point the client at a fake server.
var newHcloudClient = func() *hcloudmeta.Client {
	return hcloudmeta.NewClient()
}

// Detector is a Hetzner metadata detector.
type Detector struct {
	client *hcloudmeta.Client
	logger *zap.Logger
	rb     *metadata.ResourceBuilder
}

// NewDetector creates a new Hetzner metadata detector.
func NewDetector(p processor.Settings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)

	return &Detector{
		client: newHcloudClient(),
		logger: p.Logger,
		rb:     metadata.NewResourceBuilder(cfg.ResourceAttributes),
	}, nil
}

// Detect detects system metadata and returns a resource with the available ones.
func (d *Detector) Detect(_ context.Context) (pcommon.Resource, string, error) {
	// Quick check: if not running in Hetzner Cloud, return empty.
	if !d.client.IsHcloudServer() {
		d.logger.Debug("Hetzner detector: not running on a Hetzner Cloud server")
		return pcommon.NewResource(), "", nil
	}

	id, err := d.client.InstanceID()
	if err != nil {
		d.logger.Debug("Hetzner detector: instance ID retrieval failed", zap.Error(err))
	}

	hostname, err := d.client.Hostname()
	if err != nil {
		d.logger.Debug("Hetzner detector: hostname retrieval failed", zap.Error(err))
	}

	region, err := d.client.Region()
	if err != nil {
		d.logger.Debug("Hetzner detector: region retrieval failed", zap.Error(err))
	}

	availabilityZone, err := d.client.AvailabilityZone()
	if err != nil {
		d.logger.Debug("Hetzner detector: availability zone retrieval failed", zap.Error(err))
	}

	d.rb.SetCloudProvider(TypeStr)
	d.rb.SetHostID(fmt.Sprintf("%d", id))
	d.rb.SetHostName(hostname)
	d.rb.SetCloudRegion(region)
	d.rb.SetCloudAvailabilityZone(availabilityZone)

	return d.rb.Emit(), conventions.SchemaURL, nil
}
