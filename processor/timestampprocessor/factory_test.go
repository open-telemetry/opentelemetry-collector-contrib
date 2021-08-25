package timestampprocessor

import (
	"context"
	"fmt"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"

	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configcheck"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

func TestType(t *testing.T) {
	factory := NewFactory()
	pType := factory.Type()

	assert.Equal(t, pType, config.Type("timestamp"))
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.Equal(t, cfg, &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewID(typeStr)),
	})
	assert.NoError(t, configcheck.ValidateConfig(cfg))
}

func TestCreateProcessors(t *testing.T) {
	tests := []struct {
		configName string
		succeed    bool
	}{
		{
			configName: "config.yaml",
			succeed:    true,
		}, {
			configName: "config_invalid.yaml",
			succeed:    false,
		},
	}

	for _, test := range tests {
		factories, err := componenttest.NopFactories()
		assert.Nil(t, err)

		factory := NewFactory()
		factories.Processors[typeStr] = factory
		cfg, err := configtest.LoadConfigAndValidate(path.Join(".", "testdata", test.configName), factories)
		assert.Nil(t, err)

		for name, cfg := range cfg.Processors {
			t.Run(fmt.Sprintf("%s/%s", test.configName, name), func(t *testing.T) {
				factory := NewFactory()

				tp, tErr := factory.CreateTracesProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, consumertest.NewNop())
				// Not implemented error
				assert.NotNil(t, tErr)
				assert.Nil(t, tp)

				mp, mErr := factory.CreateMetricsProcessor(context.Background(), componenttest.NewNopProcessorCreateSettings(), cfg, consumertest.NewNop())
				assert.Equal(t, test.succeed, mp != nil)
				assert.Equal(t, test.succeed, mErr == nil)
			})
		}
	}
}
