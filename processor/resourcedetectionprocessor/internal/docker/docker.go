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
	}, nil
}

// Detect detects system metadata and returns a resource with the available ones
func (d *Detector) Detect(ctx context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	osType, err := d.provider.OSType(ctx)
	if err != nil {
		return pcommon.NewResource(), "", fmt.Errorf("failed getting OS type: %w", err)
	}

	hostname, err := d.provider.Hostname(ctx)
	if err != nil {
		return pcommon.NewResource(), "", fmt.Errorf("failed getting OS hostname: %w", err)
	}

	d.rb.SetHostName(hostname)
	d.rb.SetOsType(osType)

	info, err := d.provider.ContainerInfo(ctx)
	if err != nil {
		// Container lookup fails when the collector can't match its
		// container by hostname (for example network_mode: host). Log
		// at Debug and return the host.name / os.type we already have
		// instead of discarding the whole detection.
		d.logger.Debug("docker detector: failed getting container info; emitting host.name and os.type only", zap.Error(err))
		return d.rb.Emit(), conventions.SchemaURL, nil
	}

	d.rb.SetContainerName(info.Name)
	d.rb.SetContainerImageName(info.Image)

	return d.rb.Emit(), conventions.SchemaURL, nil
}
