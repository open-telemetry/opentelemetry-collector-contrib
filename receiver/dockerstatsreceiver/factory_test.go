// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dockerstatsreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/featuregate"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/receiver/receivertest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/dockerstatsreceiver/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	assert.Equal(t, "docker_stats", factory.Type().String())

	config := factory.CreateDefaultConfig()
	assert.NotNil(t, config, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(config))
}

func TestCreateReceiver(t *testing.T) {
	factory := NewFactory()
	config := factory.CreateDefaultConfig()

	params := receivertest.NewNopSettings(metadata.Type)
	traceReceiver, err := factory.CreateTraces(t.Context(), params, config, consumertest.NewNop())
	assert.ErrorIs(t, err, pipeline.ErrSignalNotSupported)
	assert.Nil(t, traceReceiver)

	metricReceiver, err := factory.CreateMetrics(t.Context(), params, config, consumertest.NewNop())
	assert.NoError(t, err, "Metric receiver creation failed")
	assert.NotNil(t, metricReceiver, "receiver creation failed")
}

func TestEnableSemConvMetricsFeatureGate(t *testing.T) {
	factory := NewFactory()
	config := factory.CreateDefaultConfig().(*Config)
	// Enable container.cpu.usage.system as disabled by default
	config.Metrics.ContainerCPUUsageSystem.Enabled = true
	params := receivertest.NewNopSettings(metadata.Type)

	err := featuregate.GlobalRegistry().Set("receiver.dockerstatsreceiver.enableSemConvMetrics", true)
	assert.NoError(t, err)
	receiver, err := createMetricsReceiver(t.Context(), params, config, consumertest.NewNop())
	assert.NoError(t, err, "Metric receiver creation failed")
	assert.False(t, config.Metrics.ContainerCPUUsageTotal.Enabled)
	assert.False(t, config.Metrics.ContainerCPUUsageUsermode.Enabled)
	assert.False(t, config.Metrics.ContainerCPUUsageSystem.Enabled)
	assert.False(t, config.Metrics.ContainerCPUUsageKernelmode.Enabled)
	assert.True(t, config.Metrics.ContainerCPUTime.Enabled)
	assert.False(t, config.Metrics.ContainerMemoryUsageTotal.Enabled)
	assert.True(t, config.Metrics.ContainerMemoryUsage.Enabled)

	t.Cleanup(func() {
		err := featuregate.GlobalRegistry().Set("receiver.dockerstatsreceiver.enableSemConvMetrics", false)
		assert.NoError(t, err, "Failed to reset feature gate to default state")
		assert.NoError(t, receiver.Shutdown(t.Context()), "Failed to shutdown receiver")
	})
}

func TestDisableSemConvMetricsFeatureGate(t *testing.T) {
	factory := NewFactory()
	config := factory.CreateDefaultConfig().(*Config)
	// Enable container.cpu.usage.system as disabled by default
	config.Metrics.ContainerCPUUsageSystem.Enabled = true
	params := receivertest.NewNopSettings(metadata.Type)

	err := featuregate.GlobalRegistry().Set("receiver.dockerstatsreceiver.enableSemConvMetrics", false)
	assert.NoError(t, err)
	receiver, err := createMetricsReceiver(t.Context(), params, config, consumertest.NewNop())
	assert.NoError(t, err, "Metric receiver creation failed")
	assert.True(t, config.Metrics.ContainerCPUUsageTotal.Enabled)
	assert.True(t, config.Metrics.ContainerCPUUsageUsermode.Enabled)
	assert.True(t, config.Metrics.ContainerCPUUsageSystem.Enabled)
	assert.True(t, config.Metrics.ContainerCPUUsageKernelmode.Enabled)
	assert.False(t, config.Metrics.ContainerCPUTime.Enabled)
	assert.True(t, config.Metrics.ContainerMemoryUsageTotal.Enabled)
	assert.False(t, config.Metrics.ContainerMemoryUsage.Enabled)

	t.Cleanup(func() {
		err := featuregate.GlobalRegistry().Set("receiver.dockerstatsreceiver.enableSemConvMetrics", false)
		assert.NoError(t, err, "Failed to reset feature gate to default state")
		assert.NoError(t, receiver.Shutdown(t.Context()), "Failed to shutdown receiver")
	})
}
