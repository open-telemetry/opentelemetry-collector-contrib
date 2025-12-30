// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package heroku

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"

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
	res, schemaURL, err := detector.Detect(t.Context())
	assert.Contains(t, schemaURL, "https://opentelemetry.io/schemas/")
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
	res, schemaURL, err := detector.Detect(t.Context())
	assert.Contains(t, schemaURL, "https://opentelemetry.io/schemas/")
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
	res, schemaURL, err := detector.Detect(t.Context())
	assert.Contains(t, schemaURL, "https://opentelemetry.io/schemas/")
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
	res, schemaURL, err := detector.Detect(t.Context())
	require.NoError(t, err)
	assert.Contains(t, schemaURL, "https://opentelemetry.io/schemas/")
	assert.True(t, internal.IsEmptyResource(res))
}
