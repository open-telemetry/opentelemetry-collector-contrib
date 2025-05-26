// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package heroku

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"
	conventions "go.opentelemetry.io/otel/semconv/v1.6.1"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

func TestDetectTrue(t *testing.T) {
	t.Setenv("HEROKU_DYNO_ID", "foo")
	t.Setenv("HEROKU_APP_ID", "appid")
	t.Setenv("HEROKU_APP_NAME", "appname")
	t.Setenv("HEROKU_RELEASE_CREATED_AT", "createdat")
	t.Setenv("HEROKU_RELEASE_VERSION", "v1")
	t.Setenv("HEROKU_SLUG_COMMIT", "23456")

	detector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig())
	require.NoError(t, err)
	res, schemaURL, err := detector.Detect(context.Background())
	assert.Equal(t, conventions.SchemaURL, schemaURL)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"heroku.app.id":                     "appid",
		"service.name":                      "appname",
		"service.instance.id":               "foo",
		"heroku.release.commit":             "23456",
		"heroku.release.creation_timestamp": "createdat",
		"service.version":                   "v1",
		"cloud.provider":                    "heroku",
	},
		res.Attributes().AsRaw())
}

func TestDetectTruePartial(t *testing.T) {
	t.Setenv("HEROKU_DYNO_ID", "foo")
	t.Setenv("HEROKU_APP_ID", "appid")
	t.Setenv("HEROKU_APP_NAME", "appname")
	t.Setenv("HEROKU_RELEASE_VERSION", "v1")

	detector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig())
	require.NoError(t, err)
	res, schemaURL, err := detector.Detect(context.Background())
	assert.Equal(t, conventions.SchemaURL, schemaURL)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"heroku.app.id":       "appid",
		"service.name":        "appname",
		"service.instance.id": "foo",
		"service.version":     "v1",
		"cloud.provider":      "heroku",
	},
		res.Attributes().AsRaw())
}

func TestDetectTruePartialMissingDynoId(t *testing.T) {
	t.Setenv("HEROKU_APP_ID", "appid")
	t.Setenv("HEROKU_APP_NAME", "appname")
	t.Setenv("HEROKU_RELEASE_VERSION", "v1")

	detector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig())
	require.NoError(t, err)
	res, schemaURL, err := detector.Detect(context.Background())
	assert.Equal(t, conventions.SchemaURL, schemaURL)
	require.NoError(t, err)
	assert.Equal(t, map[string]any{
		"heroku.app.id":   "appid",
		"service.name":    "appname",
		"service.version": "v1",
		"cloud.provider":  "heroku",
	},
		res.Attributes().AsRaw())
}

func TestDetectFalse(t *testing.T) {
	detector, err := NewDetector(processortest.NewNopSettings(processortest.NopType), CreateDefaultConfig())
	require.NoError(t, err)
	res, schemaURL, err := detector.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "https://opentelemetry.io/schemas/1.6.1", schemaURL)
	assert.True(t, internal.IsEmptyResource(res))
}
