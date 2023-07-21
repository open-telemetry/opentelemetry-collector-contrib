// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package heroku // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/heroku"

import (
	"context"
	"os"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/processor"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/heroku/internal/metadata"
)

const (
	// TypeStr is type of detector.
	TypeStr = "heroku"
)

// NewDetector returns a detector which can detect resource attributes on Heroku
func NewDetector(set processor.CreateSettings, dcfg internal.DetectorConfig) (internal.Detector, error) {
	cfg := dcfg.(Config)
	return &detector{
		logger:             set.Logger,
		resourceAttributes: cfg.ResourceAttributes,
	}, nil
}

type detector struct {
	logger             *zap.Logger
	resourceAttributes metadata.ResourceAttributesConfig
}

// Detect detects heroku metadata and returns a resource with the available ones
func (d *detector) Detect(_ context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	dynoID, ok := os.LookupEnv("HEROKU_DYNO_ID")
	if !ok {
		d.logger.Debug("heroku metadata unavailable", zap.Error(err))
		return pcommon.NewResource(), "", nil
	}

	rb := metadata.NewResourceBuilder(d.resourceAttributes)
	rb.SetCloudProvider("heroku")
	rb.SetServiceInstanceID(dynoID)
	if v, ok := os.LookupEnv("HEROKU_APP_ID"); ok {
		rb.SetHerokuAppID(v)
	}
	if v, ok := os.LookupEnv("HEROKU_APP_NAME"); ok {
		rb.SetServiceName(v)
	}
	if v, ok := os.LookupEnv("HEROKU_RELEASE_CREATED_AT"); ok {
		rb.SetHerokuReleaseCreationTimestamp(v)
	}
	if v, ok := os.LookupEnv("HEROKU_RELEASE_VERSION"); ok {
		rb.SetServiceVersion(v)
	}
	if v, ok := os.LookupEnv("HEROKU_SLUG_COMMIT"); ok {
		rb.SetHerokuReleaseCommit(v)
	}

	return rb.Emit(), conventions.SchemaURL, nil
}
