package httpjsonreceiver

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestNewFactory(t *testing.T) {
	factory := NewFactory()
	require.NotNil(t, factory)
	assert.Equal(t, componentType, factory.Type())
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	require.NotNil(t, cfg)

	httpjsonCfg, ok := cfg.(*Config)
	require.True(t, ok)
	assert.NotZero(t, httpjsonCfg.CollectionInterval)
	assert.NotZero(t, httpjsonCfg.Timeout)
	assert.NotNil(t, httpjsonCfg.ResourceAttributes)
}

func TestCreateMetricsReceiver(t *testing.T) {
	factory := NewFactory()
	cfg := &Config{
		Endpoints: []EndpointConfig{
			{
				URL: "http://example.com",
				Metrics: []MetricConfig{
					{
						Name:     "test_metric",
						JSONPath: "value",
					},
				},
			},
		},
	}

	consumer := consumertest.NewNop()
	settings := receivertest.NewNopSettings(componentType) // Fixed

	receiver, err := factory.CreateMetrics(
		context.Background(),
		settings,
		cfg,
		consumer,
	)

	require.NoError(t, err)
	require.NotNil(t, receiver)
}

func TestCreateMetricsReceiverInvalidConfig(t *testing.T) {
	factory := NewFactory()
	cfg := &Config{
		Endpoints: []EndpointConfig{},
	}

	consumer := consumertest.NewNop()
	settings := receivertest.NewNopSettings(componentType) // Fixed

	_, err := factory.CreateMetrics(
		context.Background(),
		settings,
		cfg,
		consumer,
	)

	assert.Error(t, err)
}
