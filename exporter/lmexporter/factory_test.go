package lmexporter

import (
	"testing"
	"context"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/component/componenttest"
)

// Test that the factory creates the default configuration
func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	assert.Equal(t, &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
	}, cfg, "failed to create default config")

	assert.NoError(t, configtest.CheckConfigStruct(cfg))
}

func TestCreateTracesExporter(t *testing.T) {
	config := &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		URL: "http://lm.example.com/api/traces",
		Headers: map[string]string{
			"x-logicmonitor-account": "xyz",
		},
		APIToken: map[string]string{
			"access_id":"rwerw232",
			"access_key":"ds#$LLFSfDFqwe-+D",
		},
	}

	params := componenttest.NewNopExporterCreateSettings()
	te, err := createTracesExporter(context.Background(), params, config)
	assert.NoError(t, err)
	assert.NotNil(t, te)
}

func TestCreateLogsExporter(t *testing.T) {
	config := &Config{
		ExporterSettings: config.NewExporterSettings(config.NewComponentID(typeStr)),
		URL: "http://lm.example.com/api/traces",
		Headers: map[string]string{
			"x-logicmonitor-account": "xyz",
		},
		APIToken: map[string]string{
			"access_id":"rwerw232",
			"access_key":"ds#$LLFSfDFqwe-+D",
		},
	}

	params := componenttest.NewNopExporterCreateSettings()
	te, err := createLogsExporter(context.Background(), params, config)
	assert.NoError(t, err)
	assert.NotNil(t, te)
}