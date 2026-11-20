// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dockerstatsreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
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

func TestCreateReceiverContainerConventionsGates(t *testing.T) {
	// The same user config is used for every case: container.cpu.usage.system is
	// opted into, container.cpu.utilization is opted out of, and the new
	// container.memory.available metric is enabled directly.
	newConfig := func() *Config {
		cfg := NewFactory().CreateDefaultConfig().(*Config)
		cfg.MetricsBuilderConfig.Metrics.ContainerCPUUsageSystem.Enabled = true
		cfg.MetricsBuilderConfig.Metrics.ContainerCPUUtilization.Enabled = false
		cfg.MetricsBuilderConfig.Metrics.ContainerMemoryAvailable.Enabled = true
		return cfg
	}
	emitBoth := func() metadata.MetricsConfig {
		metrics := newConfig().MetricsBuilderConfig.Metrics
		metrics.ContainerCPUTime.Enabled = true
		metrics.ContainerMemoryUsage.Enabled = true
		metrics.ContainerNetworkIo.Enabled = true
		metrics.ContainerMemoryAvailable.Enabled = true
		metrics.ContainerMemoryWorkingSet.Enabled = true
		metrics.ContainerDiskIo.Enabled = true
		return metrics
	}

	tests := []struct {
		name            string
		emitV1          bool
		dontEmitV0      bool
		expectedMetrics func() metadata.MetricsConfig
		expectedErr     string
	}{
		{
			name: "legacy only",
			expectedMetrics: func() metadata.MetricsConfig {
				metrics := newConfig().MetricsBuilderConfig.Metrics
				metrics.ContainerMemoryAvailable.Enabled = false
				return metrics
			},
		},
		{
			name:            "legacy and semantic conventions",
			emitV1:          true,
			expectedMetrics: emitBoth,
		},
		{
			name:       "semantic conventions only",
			emitV1:     true,
			dontEmitV0: true,
			expectedMetrics: func() metadata.MetricsConfig {
				metrics := emitBoth()
				metrics.ContainerCPUUsageTotal.Enabled = false
				metrics.ContainerCPUUsageSystem.Enabled = false
				metrics.ContainerCPUUsageKernelmode.Enabled = false
				metrics.ContainerCPUUsageUsermode.Enabled = false
				metrics.ContainerMemoryUsageTotal.Enabled = false
				metrics.ContainerNetworkIoUsageRxBytes.Enabled = false
				metrics.ContainerNetworkIoUsageTxBytes.Enabled = false
				return metrics
			},
		},
		{
			name:        "no metrics",
			dontEmitV0:  true,
			expectedErr: "feature gate receiver.docker_stats.DontEmitV0ContainerConventions requires receiver.docker_stats.EmitV1ContainerConventions to be enabled",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setGateForTest(t, metadata.ReceiverDockerStatsEmitV1ContainerConventionsFeatureGate, tt.emitV1)
			setGateForTest(t, metadata.ReceiverDockerStatsDontEmitV0ContainerConventionsFeatureGate, tt.dontEmitV0)
			cfg := newConfig()

			metricReceiver, err := NewFactory().CreateMetrics(t.Context(), receivertest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())

			if tt.expectedErr != "" {
				require.EqualError(t, err, tt.expectedErr)
				assert.Nil(t, metricReceiver)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expectedMetrics(), cfg.MetricsBuilderConfig.Metrics)
		})
	}
}
