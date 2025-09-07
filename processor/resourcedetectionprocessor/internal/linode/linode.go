// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package linode // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/linode"

import (
	"context"
	"strconv"

	linodemeta "github.com/linode/go-metadata"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/otel/semconv/v1.6.1"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/linode/internal/metadata"
)

const (
	// TypeStr is type of detector.
	TypeStr = "linode"
)

var _ internal.Detector = (*Detector)(nil)

// newLinodeClient is overridden in tests to point the client at a fake server.
var newLinodeClient = func(ctx context.Context) (linodeAPI, error) {
	return linodemeta.NewClient(ctx)
}

type linodeAPI interface {
	GetInstance(ctx context.Context) (*linodemeta.InstanceData, error)
}

// Detector is a Linode metadata detector.
type Detector struct {
	client linodeAPI
	logger *zap.Logger
	rb     *metadata.ResourceBuilder
}

// NewDetector creates a new Linode metadata detector.
func NewDetector(p processor.Settings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)

	cli, err := newLinodeClient(context.Background())
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
	// Try to fetch instance metadata; if it fails we're not on Linode (or metadata unreachable).
	inst, err := d.client.GetInstance(ctx)
	if err != nil {
		d.logger.Debug("Linode detector: not running on Linode or metadata unavailable", zap.Error(err))
		return pcommon.NewResource(), "", nil
	}

	d.rb.SetCloudAccountID(inst.AccountEUUID)
	d.rb.SetCloudProvider(TypeStr)
	d.rb.SetCloudRegion(inst.Region)
	d.rb.SetHostID(strconv.Itoa(inst.ID))
	d.rb.SetHostImageID(inst.Image.ID)
	d.rb.SetHostImageName(inst.Image.Label)
	d.rb.SetHostName(inst.Label)
	d.rb.SetHostType(inst.Type)

	return d.rb.Emit(), conventions.SchemaURL, nil
}
