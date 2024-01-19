// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package openapiprocessor // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/openapiprocessor"

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/openapiprocessor/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "testconfig.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, component.UnmarshalConfig(sub, cfg))

	assert.Equal(t,
		cfg,
		&Config{
			OpenAPISpecs:       []string{"{ \"openapi\": \"3.0.0\", \"info\": {\"title\": \"test\", \"version\": \"1.0.0\"}}"},
			OpenAPIFilePaths:   []string{"testdata/petstore.yaml"},
			OpenAPIEndpoints:   []string{"http://localhost:8080/api/v1", "http://localhost:8080/api/v2"},
			OpenAPIDirectories: []string{"http://localhost:8080"},
			APILoadTimeout:     "10s",
			APIReloadInterval:  "1m",
			AllowHTTPAndHTTPS:  true,
			Extensions:         []string{"x-foo", "x-bar"},
		},
	)
}
