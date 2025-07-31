package isolationforestprocessor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/processor/processortest"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	
	assert.NotNil(t, cfg)
	assert.NoError(t, cfg.Validate())
	
	config := cfg.(*Config)
	assert.Equal(t, 100, config.NumTrees)
	assert.Equal(t, 256, config.SubsampleSize)
	assert.Equal(t, 1000, config.WindowSize)
	assert.Equal(t, 0.6, config.AnomalyThreshold)
	assert.Equal(t, 5*time.Minute, config.TrainingInterval)
	assert.True(t, config.AddAnomalyScore)
	assert.False(t, config.DropAnomalousMetrics)
}

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	assert.NotNil(t, factory)
	assert.Equal(t, "isolationforest", factory.Type().String())
}

func TestCreateMetrics(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	
	mp, err := factory.CreateMetrics(
		processortest.NewNopCreateSettings(),
		cfg,
		nil,
	)
	
	require.Error(t, err) // Should error because nextConsumer is nil
	assert.Nil(t, mp)
}
