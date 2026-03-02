// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package libhoneyreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/libhoneyreceiver"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/confmap"
)

// getOrInsertDefault is a helper function to get or insert a default value for a configoptional.Optional type.
func getOrInsertDefault[T any](t *testing.T, opt *configoptional.Optional[T]) *T {
	if opt.HasValue() {
		return opt.Get()
	}

	empty := confmap.NewFromStringMap(map[string]any{})
	require.NoError(t, empty.Unmarshal(opt))
	val := opt.Get()
	require.NotNil(t, "Expected a default value to be set for %T", val)
	return val
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))

	libhoneyCfg, ok := cfg.(*Config)
	require.True(t, ok, "invalid Config type")

	getOrInsertDefault(t, &libhoneyCfg.HTTP)
	assert.Equal(t, "localhost:8080", libhoneyCfg.HTTP.Get().NetAddr.Endpoint)
	assert.Equal(t, []string{"/events", "/event", "/batch"}, libhoneyCfg.HTTP.Get().TracesURLPaths)
	assert.Empty(t, libhoneyCfg.AuthAPI)
	assert.Equal(t, "service.name", libhoneyCfg.FieldMapConfig.Resources.ServiceName)
	assert.Equal(t, "library.name", libhoneyCfg.FieldMapConfig.Scopes.LibraryName)
	assert.Equal(t, []string{"duration_ms"}, libhoneyCfg.FieldMapConfig.Attributes.DurationFields)
}
