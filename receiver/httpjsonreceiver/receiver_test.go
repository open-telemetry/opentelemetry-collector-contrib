package httpjsonreceiver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
)

func TestNewReceiver(t *testing.T) {
	cfg := &Config{
		CollectionInterval: 30 * time.Second,
		Timeout:            10 * time.Second,
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
	settings := receivertest.NewNopSettings(componentType) // FIXED

	receiver, err := NewReceiver(cfg, consumer, settings)
	require.NoError(t, err)
	require.NotNil(t, receiver)

	assert.Equal(t, cfg, receiver.cfg)
	assert.Equal(t, consumer, receiver.consumer)
	assert.Equal(t, settings.Logger, receiver.logger)
}

func TestReceiverStartShutdown(t *testing.T) {
	cfg := &Config{
		CollectionInterval: 1 * time.Second,
		Timeout:            1 * time.Second,
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
	settings := receivertest.NewNopSettings(componentType) // FIXED
	host := componenttest.NewNopHost()

	receiver, err := NewReceiver(cfg, consumer, settings)
	require.NoError(t, err)

	ctx := context.Background()

	// Start receiver
	err = receiver.Start(ctx, host)
	require.NoError(t, err)

	// Verify client was created
	require.NotNil(t, receiver.client)
	require.NotNil(t, receiver.scraper)

	// Shutdown receiver
	err = receiver.Shutdown(ctx)
	require.NoError(t, err)
}

func TestReceiverStartWithInitialDelay(t *testing.T) {
	cfg := &Config{
		CollectionInterval: 1 * time.Hour, // Long interval to avoid collection during test
		InitialDelay:       1 * time.Millisecond,
		Timeout:            1 * time.Second,
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
	settings := receivertest.NewNopSettings(componentType) // FIXED
	host := componenttest.NewNopHost()

	receiver, err := NewReceiver(cfg, consumer, settings)
	require.NoError(t, err)

	ctx := context.Background()

	// Start receiver
	err = receiver.Start(ctx, host)
	require.NoError(t, err)

	// Give a moment for initial delay to pass
	time.Sleep(10 * time.Millisecond)

	// Shutdown receiver
	err = receiver.Shutdown(ctx)
	require.NoError(t, err)
}
