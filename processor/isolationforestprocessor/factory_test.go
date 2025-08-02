// factory_test.go - Tests for the OpenTelemetry factory implementation
package isolationforestprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"
)

func TestFactoryType(t *testing.T) {
	factory := NewFactory()
	assert.Equal(t, typeStr, factory.Type(), "Factory should return correct type")
}

func TestFactoryCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	require.NotNil(t, cfg, "Default config should not be nil")

	// Verify it's the correct type
	processorCfg, ok := cfg.(*Config)
	require.True(t, ok, "Default config should be of type *Config")

	// Verify default configuration is valid
	err := processorCfg.Validate()
	assert.NoError(t, err, "Default configuration should be valid")

	// Verify some key defaults
	assert.Equal(t, 100, processorCfg.ForestSize)
	assert.Equal(t, "enrich", processorCfg.Mode)
	assert.Equal(t, 0.7, processorCfg.Threshold)
}

func TestFactoryCreateTracesProcessor(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	set := processortest.NewNopCreateSettings()
	nextConsumer := &consumertest.TracesSink{}

	processor, err := factory.CreateTracesProcessor(
		context.Background(),
		set,
		cfg,
		nextConsumer,
	)

	require.NoError(t, err, "Should create traces processor without error")
	require.NotNil(t, processor, "Traces processor should not be nil")

	// Verify processor capabilities
	capabilities := processor.Capabilities()
	assert.True(t, capabilities.MutatesData, "Processor should mutate data")

	// Test processor lifecycle
	err = processor.Start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err, "Should start without error")

	err = processor.Shutdown(context.Background())
	assert.NoError(t, err, "Should shutdown without error")
}

func TestFactoryCreateMetricsProcessor(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	set := processortest.NewNopCreateSettings()
	nextConsumer := &consumertest.MetricsSink{}

	processor, err := factory.CreateMetricsProcessor(
		context.Background(),
		set,
		cfg,
		nextConsumer,
	)

	require.NoError(t, err, "Should create metrics processor without error")
	require.NotNil(t, processor, "Metrics processor should not be nil")

	// Test processor lifecycle
	err = processor.Start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err, "Should start without error")

	err = processor.Shutdown(context.Background())
	assert.NoError(t, err, "Should shutdown without error")
}

func TestFactoryCreateLogsProcessor(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	set := processortest.NewNopCreateSettings()
	nextConsumer := &consumertest.LogsSink{}

	processor, err := factory.CreateLogsProcessor(
		context.Background(),
		set,
		cfg,
		nextConsumer,
	)

	require.NoError(t, err, "Should create logs processor without error")
	require.NotNil(t, processor, "Logs processor should not be nil")

	// Test processor lifecycle
	err = processor.Start(context.Background(), componenttest.NewNopHost())
	assert.NoError(t, err, "Should start without error")

	err = processor.Shutdown(context.Background())
	assert.NoError(t, err, "Should shutdown without error")
}

func TestFactoryCreateWithInvalidConfig(t *testing.T) {
	factory := NewFactory()

	// Create invalid configuration
	cfg := &Config{
		ForestSize: -1, // Invalid
	}

	set := processortest.NewNopCreateSettings()

	// Test traces processor creation with invalid config
	_, err := factory.CreateTracesProcessor(
		context.Background(),
		set,
		cfg,
		&consumertest.TracesSink{},
	)
	assert.Error(t, err, "Should fail to create traces processor with invalid config")

	// Test metrics processor creation with invalid config
	_, err = factory.CreateMetricsProcessor(
		context.Background(),
		set,
		cfg,
		&consumertest.MetricsSink{},
	)
	assert.Error(t, err, "Should fail to create metrics processor with invalid config")

	// Test logs processor creation with invalid config
	_, err = factory.CreateLogsProcessor(
		context.Background(),
		set,
		cfg,
		&consumertest.LogsSink{},
	)
	assert.Error(t, err, "Should fail to create logs processor with invalid config")
}

func TestFactoryCreateWithWrongConfigType(t *testing.T) {
	factory := NewFactory()

	// Use wrong config type
	wrongConfig := struct{}{}

	set := processortest.NewNopCreateSettings()

	_, err := factory.CreateTracesProcessor(
		context.Background(),
		set,
		wrongConfig,
		&consumertest.TracesSink{},
	)
	assert.Error(t, err, "Should fail with wrong config type")
	assert.Contains(t, err.Error(), "configuration is not of type *Config")
}

// ---
