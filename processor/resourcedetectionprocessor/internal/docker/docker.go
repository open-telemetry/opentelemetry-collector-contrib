// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package docker // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/docker"

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
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
func NewDetector(p processor.CreateSettings, cfg internal.DetectorConfig) (internal.Detector, error) {
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

	return d.rb.Emit(), conventions.SchemaURL, nil
}
