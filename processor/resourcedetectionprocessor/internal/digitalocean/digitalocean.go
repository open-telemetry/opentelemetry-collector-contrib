// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package digitalocean // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/digitalocean"

import (
	"context"
	"fmt"

	do "github.com/digitalocean/go-metadata"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/otel/semconv/v1.39.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/digitalocean/internal/metadata"
)

const (
	// TypeStr is type of detector.
	TypeStr = "digitalocean"
)

var _ internal.Detector = (*Detector)(nil)

// newDigitalOceanClient is overridden in tests to point the client at a fake server.
var newDigitalOceanClient = func() *do.Client {
	return do.NewClient()
}

// Detector is a DigitalOcean metadata detector.
type Detector struct {
	client *do.Client
	logger *zap.Logger
	rb     *metadata.ResourceBuilder
}

// NewDetector creates a new DigitalOcean metadata detector.
func NewDetector(p processor.Settings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)

	return &Detector{
		client: newDigitalOceanClient(),
		logger: p.Logger,
		rb:     metadata.NewResourceBuilder(cfg.ResourceAttributes),
	}, nil
}

// Detect detects system metadata and returns a resource with the available ones.
func (d *Detector) Detect(_ context.Context) (pcommon.Resource, string, error) {
	md, err := d.client.Metadata()
	if err != nil || md == nil {
		d.logger.Debug("DigitalOcean detector: not running on DigitalOcean or metadata unavailable", zap.Error(err))
		return pcommon.NewResource(), "", nil
	}

	d.rb.SetCloudProvider(TypeStr)
	d.rb.SetHostID(fmt.Sprintf("%d", md.DropletID))
	d.rb.SetHostName(md.Hostname)
	d.rb.SetCloudRegion(md.Region)

	return d.rb.Emit(), conventions.SchemaURL, nil
}
