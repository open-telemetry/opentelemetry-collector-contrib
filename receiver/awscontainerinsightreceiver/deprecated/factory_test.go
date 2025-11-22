// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package deprecated

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/receiver/receivertest"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscontainerinsightreceiver"
)

func TestNewFactory(t *testing.T) {
	f := NewFactory()
	assert.NotNil(t, f)
	assert.Equal(t, component.MustNewType("awscontainerinsightreceiver"), f.Type())
}

func TestCreateMetricsReceiver(t *testing.T) {
	f := NewFactory()
	cfg := &awscontainerinsightreceiver.Config{
		CollectionInterval:        60,
		ContainerOrchestrator:     "eks",
		TagService:                true,
		PrefFullPodName:           false,
		AddFullPodNameMetricLabel: false,
	}

	params := receivertest.NewNopSettings(f.Type())

	metricsReceiver, err := f.CreateMetrics(t.Context(), params, cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, metricsReceiver)
}

func TestDeprecationWarningOnStart(t *testing.T) {
	// Set up a logger that captures logs
	core, observed := observer.New(zap.WarnLevel)
	logger := zap.New(core)

	f := NewFactory()
	cfg := &awscontainerinsightreceiver.Config{
		CollectionInterval:        60,
		ContainerOrchestrator:     "eks",
		TagService:                true,
		PrefFullPodName:           false,
		AddFullPodNameMetricLabel: false,
	}

	params := receivertest.NewNopSettings(f.Type())
	params.Logger = logger

	metricsReceiver, err := f.CreateMetrics(t.Context(), params, cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, metricsReceiver)

	// Verify it's a wrapper
	wrapper, ok := metricsReceiver.(*deprecatedReceiverWrapper)
	require.True(t, ok, "Receiver should be wrapped with deprecatedReceiverWrapper")
	assert.NotNil(t, wrapper.Metrics, "Wrapped receiver should not be nil")
	assert.NotNil(t, wrapper.logger, "Logger should be set")

	// Test that the wrapper's Start method logs the deprecation warning
	// Create a mock receiver that implements receiver.Metrics
	mockReceiver := &mockMetricsReceiver{}
	wrapper.Metrics = mockReceiver

	// Call Start and verify the warning is logged
	err = wrapper.Start(t.Context(), nil)
	require.NoError(t, err)

	// Check that the deprecation warning was logged
	warningFound := false
	for _, log := range observed.All() {
		if log.Level == zap.WarnLevel {
			msg := log.Message
			if assert.Contains(t, msg, "deprecated") && assert.Contains(t, msg, "awscontainerinsightreceiver") {
				warningFound = true
				break
			}
		}
	}
	assert.True(t, warningFound, "Deprecation warning should be logged on Start")
}

// mockMetricsReceiver is a simple mock that implements receiver.Metrics for testing
type mockMetricsReceiver struct{}

func (_ *mockMetricsReceiver) Start(context.Context, component.Host) error { return nil }
func (_ *mockMetricsReceiver) Shutdown(context.Context) error              { return nil }
