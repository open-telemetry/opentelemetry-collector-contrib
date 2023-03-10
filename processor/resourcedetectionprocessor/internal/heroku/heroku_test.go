// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	d, err := NewDetector(processortest.NewNopCreateSettings(), nil)
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

	detector := &detector{}
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

	detector := &detector{}
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
