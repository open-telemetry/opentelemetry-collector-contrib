package honeycombauthextension

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/service/servicetest"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factory := NewFactory()
	factories.Extensions[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)

	require.Nil(t, err)
	require.NotNil(t, cfg)

	ext := cfg.Extensions[config.NewComponentID(typeStr)]
	expected := factory.CreateDefaultConfig().(*Config)
	expected.Team = "test-team"
	expected.Dataset = "test-dataset"
	assert.Equal(t, expected, ext)

	assert.Equal(t, 1, len(cfg.Service.Extensions))
	assert.Equal(t, config.NewComponentID(typeStr), cfg.Service.Extensions[0])
}

func TestLoadConfigFromEnv(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	os.Setenv(teamEnvKey, "test-env-team")
	os.Setenv(datasetEnvKey, "test-env-dataset")

	factory := NewFactory()
	factories.Extensions[typeStr] = factory
	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config_env.yaml"), factories)

	require.Nil(t, err)
	require.NotNil(t, cfg)

	ext := cfg.Extensions[config.NewComponentID(typeStr)]
	expected := factory.CreateDefaultConfig().(*Config)
	expected.Team = "test-env-team"
	expected.Dataset = "test-env-dataset"
	assert.Equal(t, expected, ext)

	assert.Equal(t, 1, len(cfg.Service.Extensions))
	assert.Equal(t, config.NewComponentID(typeStr), cfg.Service.Extensions[0])
}

func TestLoadConfigError(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	tests := []struct {
		configName  string
		expectedErr error
	}{
		{
			"missing-dataset",
			errNoDatasetProvided,
		},
		{
			"missing-team",
			errNoTeamProvided,
		},
	}
	for _, tt := range tests {
		factory := NewFactory()
		factories.Extensions[typeStr] = factory
		cfg, _ := servicetest.LoadConfig(filepath.Join("testdata", "config_bad.yaml"), factories)
		extension := cfg.Extensions[config.NewComponentIDWithName(typeStr, tt.configName)]
		err := extension.Validate()
		require.ErrorIs(t, err, tt.expectedErr)
	}
}
