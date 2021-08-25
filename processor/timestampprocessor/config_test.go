package timestampprocessor

import (
	"path"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
)

// TestLoadingConfigRegexp tests loading testdata/config_strict.yaml
func TestLoadingConfigStrict(t *testing.T) {
	factories, err := componenttest.NopFactories()
	assert.Nil(t, err)

	factory := NewFactory()
	factories.Processors[typeStr] = factory
	cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", "config.yaml"), factories)

	assert.Nil(t, err)
	require.NotNil(t, cfg)

	oneSecond := time.Second

	tests := []struct {
		filterID config.ComponentID
		expCfg   *Config
	}{
		{
			filterID: config.NewIDWithName("timestamp", "1sec"),
			expCfg: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewIDWithName(typeStr, "1sec")),
				RoundToNearest:    &oneSecond,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.filterID.String(), func(t *testing.T) {
			cfg := cfg.Processors[test.filterID]
			assert.Equal(t, test.expCfg, cfg)
		})
	}
}
