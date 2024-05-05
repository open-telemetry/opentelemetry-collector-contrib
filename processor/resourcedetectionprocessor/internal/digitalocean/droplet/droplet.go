// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package droplet // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/digitalocean/droplet"

import (
	"context"
	"strconv"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"

	dometa "github.com/digitalocean/go-metadata"

	dropletprovider "github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/digitalocean/droplet"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/digitalocean/droplet/internal/metadata"
)

const (
	// TypeStr is type of detector.
	TypeStr = "droplet"
)

var _ internal.Detector = (*Detector)(nil)

type Detector struct {
	metadataProvider dropletprovider.Provider
	logger           *zap.Logger
	rb               *metadata.ResourceBuilder
}

func NewDetector(set processor.CreateSettings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)

	return &Detector{
		metadataProvider: dropletprovider.NewProvider(dometa.NewClient()),
		logger:           set.Logger,
		rb:               metadata.NewResourceBuilder(cfg.ResourceAttributes),
	}, nil
}

func (d *Detector) Detect(ctx context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	metadata, err := d.metadataProvider.Get(ctx)

	if err != nil {
		d.logger.Debug("Droplet metadata unavailable", zap.Error(err))
		return pcommon.NewResource(), "", nil
	}

	d.rb.SetCloudProvider("digital_ocean")
	d.rb.SetCloudPlatform("droplet")
	d.rb.SetCloudRegion(metadata.Region)
	d.rb.SetCloudResourceID(strconv.Itoa(metadata.DropletID))
	d.rb.SetHostID(strconv.Itoa(metadata.DropletID))
	d.rb.SetHostName(metadata.Hostname)
	res := d.rb.Emit()

	return res, conventions.SchemaURL, nil
}
