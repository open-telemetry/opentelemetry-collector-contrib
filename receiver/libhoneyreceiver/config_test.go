// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package libhoneyreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))

	libhoneyCfg, ok := cfg.(*Config)
	require.True(t, ok, "invalid Config type")

	assert.Equal(t, "localhost:8080", libhoneyCfg.HTTP.Endpoint)
	assert.Equal(t, []string{"/events", "/event", "/batch"}, libhoneyCfg.HTTP.TracesURLPaths)
	assert.Empty(t, libhoneyCfg.AuthAPI)
	assert.Equal(t, "service.name", libhoneyCfg.FieldMapConfig.Resources.ServiceName)
	assert.Equal(t, "library.name", libhoneyCfg.FieldMapConfig.Scopes.LibraryName)
	assert.Equal(t, []string{"duration_ms"}, libhoneyCfg.FieldMapConfig.Attributes.DurationFields)
}
