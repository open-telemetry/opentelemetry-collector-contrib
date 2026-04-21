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

	// Container lookup was introduced in #44898 and became a hard failure
	// in v0.145. For containers that run in `network_mode: host` (or with
	// a custom hostname) the hostname inside the container does not match
	// the container's name, so ContainerInfo returns "No such container"
	// and the entire resourcedetection processor fails to start — even
	// when the operator only wants host.name / os.type on their telemetry
	// (#46275). Log the failure and continue emitting the host-level
	// attributes we already collected instead of aborting detection.
	info, err := d.provider.ContainerInfo(ctx)
	if err != nil {
		d.logger.Debug("failed getting container info; skipping container.* attributes", zap.Error(err))
	} else {
		d.rb.SetContainerName(info.Name)
		d.rb.SetContainerImageName(info.Image)
	}

	return d.rb.Emit(), conventions.SchemaURL, nil
}
