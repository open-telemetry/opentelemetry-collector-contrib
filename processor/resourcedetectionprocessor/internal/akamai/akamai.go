// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package akamai // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/akamai"

import (
	"context"
	"strconv"

	linodemeta "github.com/linode/go-metadata"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/otel/semconv/v1.38.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/akamai/internal/metadata"
)

const (
	// TypeStr is type of detector.
	TypeStr = "akamai"
)

var _ internal.Detector = (*Detector)(nil)

// newAkamaiClient is overridden in tests to point the client at a fake server.
var newAkamaiClient = func(ctx context.Context) (akamaiAPI, error) {
	return linodemeta.NewClient(ctx)
}

type akamaiAPI interface {
	GetInstance(ctx context.Context) (*linodemeta.InstanceData, error)
}

// Detector is a Akamai metadata detector.
type Detector struct {
	client akamaiAPI
	logger *zap.Logger
	rb     *metadata.ResourceBuilder
}

// NewDetector creates a new Akamai metadata detector.
func NewDetector(p processor.Settings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)

	cli, err := newAkamaiClient(context.Background())
	if err != nil {
		return nil, err
	}

	return &Detector{
		client: cli,
		logger: p.Logger,
		rb:     metadata.NewResourceBuilder(cfg.ResourceAttributes),
	}, nil
}

// Detect detects system metadata and returns a resource with the available ones.
func (d *Detector) Detect(ctx context.Context) (pcommon.Resource, string, error) {
	// Try to fetch instance metadata; if it fails we're not on Akamai (or metadata unreachable).
	inst, err := d.client.GetInstance(ctx)
	if err != nil {
		d.logger.Debug("Akamai detector: not running on Akamai or metadata unavailable", zap.Error(err))
		return pcommon.NewResource(), "", nil
	}

	d.rb.SetCloudAccountID(inst.AccountEUUID)
	// Cloud provider and platform values will be "akamai_cloud" and "akamai_cloud_platform" from conventions when it's merged.
	// d.rb.SetCloudProvider(conventions.CloudProviderAkamaiCloud.Value.AsString())
	// d.rb.SetCloudPlatform(conventions.CloudPlatformAkamaiCloud.Value.AsString())
	d.rb.SetCloudPlatform("akamai_cloud_platform")
	d.rb.SetCloudProvider("akamai_cloud")
	d.rb.SetCloudRegion(inst.Region)
	d.rb.SetHostID(strconv.Itoa(inst.ID))
	d.rb.SetHostImageID(inst.Image.ID)
	d.rb.SetHostImageName(inst.Image.Label)
	d.rb.SetHostName(inst.Label)
	d.rb.SetHostType(inst.Type)

	return d.rb.Emit(), conventions.SchemaURL, nil
}
