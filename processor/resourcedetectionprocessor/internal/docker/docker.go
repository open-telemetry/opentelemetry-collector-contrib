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
}

// NewDetector creates a new system metadata detector
func NewDetector(p processor.CreateSettings, _ internal.DetectorConfig) (internal.Detector, error) {
	dockerProvider, err := docker.NewProvider()
	if err != nil {
		return nil, fmt.Errorf("failed creating detector: %w", err)
	}

	return &Detector{provider: dockerProvider, logger: p.Logger}, nil
}

// Detect detects system metadata and returns a resource with the available ones
func (d *Detector) Detect(ctx context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	res := pcommon.NewResource()
	attrs := res.Attributes()

	osType, err := d.provider.OSType(ctx)
	if err != nil {
		return res, "", fmt.Errorf("failed getting OS type: %w", err)
	}

	hostname, err := d.provider.Hostname(ctx)
	if err != nil {
		return res, "", fmt.Errorf("failed getting OS hostname: %w", err)
	}

	attrs.PutStr(conventions.AttributeHostName, hostname)
	attrs.PutStr(conventions.AttributeOSType, osType)

	return res, conventions.SchemaURL, nil
}
