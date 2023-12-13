package solarwindsapmsettingsextension

import (
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"testing"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	ocfg, ok := factory.CreateDefaultConfig().(*Config)
	assert.True(t, ok)
	assert.Empty(t, ocfg.Endpoint, "There is no default endpoint")
	assert.Empty(t, ocfg.Key, "There is no default key")
	assert.Equal(t, ocfg.Interval, DefaultInterval, "Wrong default interval")
}
