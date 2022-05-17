package promtailreceiver

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/service/servicetest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/stanza"
)

func TestLoadConfig(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.NoError(t, err)

	factories.Receivers[typeStr] = NewFactory()

	cfg, err := servicetest.LoadConfigAndValidate(filepath.Join("testdata", "config.yaml"), factories)
	require.NoError(t, err)
	require.NotNil(t, cfg)

	assert.Equal(t, len(cfg.Receivers), 1)
	assert.Equal(t, testdataConfigYamlAsMap(), cfg.Receivers[config.NewComponentID("promtail")])
}

func testdataConfigYamlAsMap() *PromtailConfig {
	return &PromtailConfig{
		BaseConfig: stanza.BaseConfig{
			ReceiverSettings: config.NewReceiverSettings(config.NewComponentID(typeStr)),
			Operators:        stanza.OperatorConfigs{},
		},
		Input: stanza.InputConfig{
			"config": map[string]interface{}{
				"positions": map[string]interface{}{
					"filename": "/tmp/positions.yaml",
				},
				"scrape_configs": []interface{}{
					map[string]interface{}{
						"job_name": "system",
						"static_configs": []interface{}{
							map[string]interface{}{
								"labels": map[string]interface{}{
									"job":      "varlogs",
									"__path__": "testdata/simple.log",
								},
							},
						},
					},
				},
			},
		},
	}
}
