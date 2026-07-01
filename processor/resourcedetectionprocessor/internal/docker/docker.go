// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package docker // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/docker"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/metadataproviders/docker"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/docker/internal/metadata"
)

const (
	// TypeStr is type of detector.
	TypeStr = "docker"
)

var _ internal.Detector = (*Detector)(nil)

// Detector is a system metadata detector
type Detector struct {
	provider docker.Provider
	logger   *zap.Logger
	rb       *metadata.ResourceBuilder
	cfg      metadata.ResourceAttributesConfig
}

// NewDetector creates a new system metadata detector
func NewDetector(p processor.Settings, cfg internal.DetectorConfig) (internal.Detector, error) {
	dockerProvider, err := docker.NewProvider()
	if err != nil {
		return nil, fmt.Errorf("failed creating detector: %w", err)
	}

	return &Detector{
		provider: dockerProvider,
		logger:   p.Logger,
		rb:       metadata.NewResourceBuilder(cfg.(Config).ResourceAttributes),
		cfg:      cfg.(Config).ResourceAttributes,
	}, nil
}

// Detect detects system metadata and returns a resource with the available ones
func (d *Detector) Detect(ctx context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	if d.cfg.OsType.Enabled {
		osType, err := d.provider.OSType(ctx)
		if err != nil {
			return pcommon.NewResource(), "", fmt.Errorf("failed getting OS type: %w", err)
		}
		d.rb.SetOsType(osType)
	}

	if d.cfg.HostName.Enabled {
		hostname, err := d.provider.Hostname(ctx)
		if err != nil {
			return pcommon.NewResource(), "", fmt.Errorf("failed getting OS hostname: %w", err)
		}
		d.rb.SetHostName(hostname)
	}

	if d.cfg.ContainerName.Enabled || d.cfg.ContainerImageName.Enabled {
		info, err := d.provider.ContainerInfo(ctx)
		if err != nil {
			return pcommon.NewResource(), "", fmt.Errorf("failed getting container info: %w", err)
		}
		d.rb.SetContainerName(info.Name)
		d.rb.SetContainerImageName(info.Image)
	}

	return d.rb.Emit(), conventions.SchemaURL, nil
}
