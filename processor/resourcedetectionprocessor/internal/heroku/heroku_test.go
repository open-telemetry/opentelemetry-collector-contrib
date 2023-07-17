// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package heroku

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"
	conventions "go.opentelemetry.io/collector/semconv/v1.6.1"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal"
)

func TestNewDetector(t *testing.T) {
	dcfg := CreateDefaultConfig()
	d, err := NewDetector(processortest.NewNopCreateSettings(), dcfg)
	assert.NotNil(t, d)
	assert.NoError(t, err)
}

func TestDetectTrue(t *testing.T) {
	t.Setenv("HEROKU_DYNO_ID", "foo")
	t.Setenv("HEROKU_APP_ID", "appid")
	t.Setenv("HEROKU_APP_NAME", "appname")
	t.Setenv("HEROKU_RELEASE_CREATED_AT", "createdat")
	t.Setenv("HEROKU_RELEASE_VERSION", "v1")
	t.Setenv("HEROKU_SLUG_COMMIT", "23456")

	resourceAttributes := CreateDefaultConfig().ResourceAttributes
	detector := &detector{resourceAttributes: resourceAttributes}
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

	resourceAttributes := CreateDefaultConfig().ResourceAttributes
	detector := &detector{resourceAttributes: resourceAttributes}
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

func TestDetectFalse(t *testing.T) {

	detector := &detector{
		logger: zap.NewNop(),
	}
	res, schemaURL, err := detector.Detect(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "", schemaURL)
	assert.True(t, internal.IsEmptyResource(res))
}
