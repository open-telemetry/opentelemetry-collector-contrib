package logzioexporter

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config/configmodels"
	"go.opentelemetry.io/collector/config/configtest"
	"path"
	"testing"
)

func TestLoadConfig(tester *testing.T) {
	factories, err := componenttest.ExampleComponents()
	assert.Nil(tester, err)

	factory := NewFactory()
	factories.Exporters[configmodels.Type(typeStr)] = factory
	cfg, err := configtest.LoadConfigFile(
		tester, path.Join(".", "testdata", "config.yaml"), factories,
	)

	require.NoError(tester, err)
	require.NotNil(tester, cfg)

	assert.Equal(tester, len(cfg.Exporters), 1)

	config := cfg.Exporters["logzio"].(*Config)
	assert.Equal(tester, config, &Config{
		ExporterSettings: configmodels.ExporterSettings{TypeVal: typeStr, NameVal: typeStr},
		Token:            "logzioTESTtoken",
		Region:           "eu",
	})
}
