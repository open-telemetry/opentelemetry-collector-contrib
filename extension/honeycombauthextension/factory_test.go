package honeycombauthextension

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
	"testing"
)

func TestFactory_CreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.Equal(t, &Config{ExtensionSettings: config.NewExtensionSettings(config.NewComponentID(typeStr))}, cfg)
	assert.NoError(t, configtest.CheckConfigStruct(cfg))
}

func TestFactory_CreateExtension(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Team = "test-team"
	cfg.Dataset = "test-dataset"
	ext, err := createExtension(context.Background(), componenttest.NewNopExtensionCreateSettings(), cfg)
	require.NoError(t, err)
	require.NotNil(t, ext)
}
