// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package render // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/render"

import (
	"context"
	"os"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/otel/semconv/v1.40.0"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/render/internal/metadata"
)

const (
	// TypeStr is type of detector.
	TypeStr = "render"
)

// NewDetector returns a detector which can detect resource attributes on Render.com
func NewDetector(set processor.Settings, dcfg internal.DetectorConfig, _ bool) (internal.Detector, error) {
	cfg := dcfg.(Config)
	return &detector{
		logger: set.Logger,
		rb:     metadata.NewResourceBuilder(cfg.ResourceAttributes),
	}, nil
}

type detector struct {
	logger *zap.Logger
	rb     *metadata.ResourceBuilder
}

// Detect detects Render.com metadata and returns a resource with the available ones
func (d *detector) Detect(_ context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	serviceID, ok := os.LookupEnv("RENDER_SERVICE_ID")
	if !ok {
		d.logger.Debug("Render metadata is missing. RENDER_SERVICE_ID not set.")
		return pcommon.NewResource(), "", nil
	}

	if v, ok := os.LookupEnv("RENDER_INSTANCE_ID"); ok {
		d.rb.SetServiceInstanceID(v)
	}
	if v, ok := os.LookupEnv("RENDER_SERVICE_NAME"); ok {
		d.rb.SetServiceName(v)
	} else {
		d.rb.SetServiceName(serviceID)
	}
	if v, ok := os.LookupEnv("RENDER_GIT_COMMIT"); ok {
		d.rb.SetServiceVersion(v)
	}

	return d.rb.Emit(), conventions.SchemaURL, nil
}
