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
)

const (
	// TypeStr is type of detector.
	TypeStr = "heroku"

	// The time and date the release was created.
	herokuReleaseCreationTimestamp = "heroku.release.creation_timestamp"
	// The commit hash for the current release
	herokuReleaseCommit = "heroku.release.commit"
	// The unique identifier for the application
	herokuAppID = "heroku.app.id"
)

// NewDetector returns a detector which can detect resource attributes on Heroku
func NewDetector(set processor.CreateSettings, _ internal.DetectorConfig) (internal.Detector, error) {
	return &detector{
		logger: set.Logger,
	}, nil
}

type detector struct {
	logger *zap.Logger
}

// Detect detects heroku metadata and returns a resource with the available ones
func (d *detector) Detect(_ context.Context) (resource pcommon.Resource, schemaURL string, err error) {
	res := pcommon.NewResource()
	dynoID, ok := os.LookupEnv("HEROKU_DYNO_ID")
	if !ok {
		d.logger.Debug("heroku metadata unavailable", zap.Error(err))
		return res, "", nil
	}

	attrs := res.Attributes()
	attrs.PutStr(conventions.AttributeCloudProvider, "heroku")

	attrs.PutStr(conventions.AttributeServiceInstanceID, dynoID)
	if v, ok := os.LookupEnv("HEROKU_APP_ID"); ok {
		attrs.PutStr(herokuAppID, v)
	}
	if v, ok := os.LookupEnv("HEROKU_APP_NAME"); ok {
		attrs.PutStr(conventions.AttributeServiceName, v)
	}
	if v, ok := os.LookupEnv("HEROKU_RELEASE_CREATED_AT"); ok {
		attrs.PutStr(herokuReleaseCreationTimestamp, v)
	}
	if v, ok := os.LookupEnv("HEROKU_RELEASE_VERSION"); ok {
		attrs.PutStr(conventions.AttributeServiceVersion, v)
	}
	if v, ok := os.LookupEnv("HEROKU_SLUG_COMMIT"); ok {
		attrs.PutStr(herokuReleaseCommit, v)
	}

	return res, conventions.SchemaURL, nil
}
