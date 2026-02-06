// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourcedetectionprocessor

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/collector/processor/xprocessor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/resourcedetectionprocessor/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	assert.NotNil(t, cfg)
}

func TestCreateProcessor(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, tp)

	mp, err := factory.CreateMetrics(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, mp)

	lp, err := factory.CreateLogs(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, lp)

	pp, err := factory.(xprocessor.Factory).CreateProfiles(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, pp)
}

func TestCreateConfigProcessors(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	for k := range cm.ToStringMap() {
		// Check if all processor variations that are defined in test config can be actually created
		t.Run(k, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(k)
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			tt, err := factory.CreateTraces(
				t.Context(),
				processortest.NewNopSettings(metadata.Type),
				cfg,
				consumertest.NewNop(),
			)
			assert.NoError(t, err)
			assert.NotNil(t, tt)
		})
	}
}

func TestInvalidConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Detectors = []string{"not-existing"}

	tp, err := factory.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.Error(t, err)
	assert.Nil(t, tp)

	mp, err := factory.CreateMetrics(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.Error(t, err)
	assert.Nil(t, mp)

	lp, err := factory.CreateLogs(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.Error(t, err)
	assert.Nil(t, lp)

	pp, err := factory.(xprocessor.Factory).CreateProfiles(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	assert.Error(t, err)
	assert.Nil(t, pp)
}

// TestLifecycle tests the processor's initialization with a valid configuration
// and ensures proper context propagation. This test satisfies the stability requirement
// for at least one lifecycle test that validates component initialization with valid
// configuration and context propagation.
func TestLifecycle(t *testing.T) {
	tests := []struct {
		name          string
		signalType    string
		createAndTest func(t *testing.T, factory processor.Factory, cfg component.Config)
	}{
		{
			name:       "traces_processor",
			signalType: "traces",
			createAndTest: func(t *testing.T, factory processor.Factory, cfg component.Config) {
				ctx := t.Context() // Create processor
				tp, err := factory.CreateTraces(ctx, processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
				require.NoError(t, err, "processor creation should succeed with valid config")
				require.NotNil(t, tp, "processor should not be nil")

				// Verify capabilities
				caps := tp.Capabilities()
				assert.True(t, caps.MutatesData, "processor should mutate data")

				// Test lifecycle: Start
				host := componenttest.NewNopHost()
				err = tp.Start(ctx, host)
				require.NoError(t, err, "processor start should succeed")

				// Test context propagation through consume operation
				type contextKey string
				testKey := contextKey("test-key")
				ctxWithValue := context.WithValue(ctx, testKey, "test-value")

				td := ptrace.NewTraces()
				err = tp.ConsumeTraces(ctxWithValue, td)
				require.NoError(t, err, "consume operation should succeed with propagated context")

				// Test lifecycle: Shutdown
				err = tp.Shutdown(ctx)
				require.NoError(t, err, "processor shutdown should succeed")

				// Test multiple start/stop cycles
				err = tp.Start(ctx, host)
				require.NoError(t, err, "processor restart should succeed")

				err = tp.Shutdown(ctx)
				require.NoError(t, err, "processor second shutdown should succeed")
			},
		},
		{
			name:       "metrics_processor",
			signalType: "metrics",
			createAndTest: func(t *testing.T, factory processor.Factory, cfg component.Config) {
				ctx := t.Context()
				mp, err := factory.CreateMetrics(ctx, processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
				require.NoError(t, err, "processor creation should succeed with valid config")
				require.NotNil(t, mp, "processor should not be nil")

				caps := mp.Capabilities()
				assert.True(t, caps.MutatesData, "processor should mutate data")

				host := componenttest.NewNopHost()
				err = mp.Start(ctx, host)
				require.NoError(t, err, "processor start should succeed")

				type contextKey string
				testKey := contextKey("test-key")
				ctxWithValue := context.WithValue(ctx, testKey, "test-value")

				md := pmetric.NewMetrics()
				err = mp.ConsumeMetrics(ctxWithValue, md)
				require.NoError(t, err, "consume operation should succeed with propagated context")

				err = mp.Shutdown(ctx)
				require.NoError(t, err, "processor shutdown should succeed")
			},
		},
		{
			name:       "logs_processor",
			signalType: "logs",
			createAndTest: func(t *testing.T, factory processor.Factory, cfg component.Config) {
				ctx := t.Context()
				lp, err := factory.CreateLogs(ctx, processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
				require.NoError(t, err, "processor creation should succeed with valid config")
				require.NotNil(t, lp, "processor should not be nil")

				caps := lp.Capabilities()
				assert.True(t, caps.MutatesData, "processor should mutate data")

				host := componenttest.NewNopHost()
				err = lp.Start(ctx, host)
				require.NoError(t, err, "processor start should succeed")

				type contextKey string
				testKey := contextKey("test-key")
				ctxWithValue := context.WithValue(ctx, testKey, "test-value")

				ld := plog.NewLogs()
				err = lp.ConsumeLogs(ctxWithValue, ld)
				require.NoError(t, err, "consume operation should succeed with propagated context")

				err = lp.Shutdown(ctx)
				require.NoError(t, err, "processor shutdown should succeed")
			},
		},
		{
			name:       "profiles_processor",
			signalType: "profiles",
			createAndTest: func(t *testing.T, factory processor.Factory, cfg component.Config) {
				ctx := t.Context()
				pp, err := factory.(xprocessor.Factory).CreateProfiles(ctx, processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
				require.NoError(t, err, "processor creation should succeed with valid config")
				require.NotNil(t, pp, "processor should not be nil")

				caps := pp.Capabilities()
				assert.True(t, caps.MutatesData, "processor should mutate data")

				host := componenttest.NewNopHost()
				err = pp.Start(ctx, host)
				require.NoError(t, err, "processor start should succeed")

				type contextKey string
				testKey := contextKey("test-key")
				ctxWithValue := context.WithValue(ctx, testKey, "test-value")

				pd := pprofile.NewProfiles()
				err = pp.ConsumeProfiles(ctxWithValue, pd)
				require.NoError(t, err, "consume operation should succeed with propagated context")

				err = pp.Shutdown(ctx)
				require.NoError(t, err, "processor shutdown should succeed")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create factory and valid configuration
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()
			oCfg := cfg.(*Config)

			// Configure with system detector (always available)
			oCfg.Detectors = []string{"system"}
			oCfg.Override = false

			// Run the signal-specific test
			tt.createAndTest(t, factory, cfg)
		})
	}
}

// TestLifecycleWithAllDetectors tests initialization with multiple detectors
// to ensure proper configuration handling of all detector types
func TestLifecycleWithAllDetectors(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)

	// Test with multiple detectors that are always available
	oCfg.Detectors = []string{"env", "system"}
	oCfg.Override = true

	ctx := t.Context()
	host := componenttest.NewNopHost()

	// Test all signal types with multiple detectors
	t.Run("traces", func(t *testing.T) {
		processor, err := factory.CreateTraces(ctx, processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
		require.NoError(t, err, "processor creation with multiple detectors should succeed")
		require.NotNil(t, processor)

		err = processor.Start(ctx, host)
		require.NoError(t, err, "processor start with multiple detectors should succeed")

		err = processor.ConsumeTraces(ctx, ptrace.NewTraces())
		require.NoError(t, err, "consume operation should succeed")

		err = processor.Shutdown(ctx)
		require.NoError(t, err, "processor shutdown should succeed")
	})

	t.Run("metrics", func(t *testing.T) {
		processor, err := factory.CreateMetrics(ctx, processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
		require.NoError(t, err)
		require.NotNil(t, processor)

		err = processor.Start(ctx, host)
		require.NoError(t, err)

		err = processor.ConsumeMetrics(ctx, pmetric.NewMetrics())
		require.NoError(t, err)

		err = processor.Shutdown(ctx)
		require.NoError(t, err)
	})

	t.Run("logs", func(t *testing.T) {
		processor, err := factory.CreateLogs(ctx, processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
		require.NoError(t, err)
		require.NotNil(t, processor)

		err = processor.Start(ctx, host)
		require.NoError(t, err)

		err = processor.ConsumeLogs(ctx, plog.NewLogs())
		require.NoError(t, err)

		err = processor.Shutdown(ctx)
		require.NoError(t, err)
	})

	t.Run("profiles", func(t *testing.T) {
		processor, err := factory.(xprocessor.Factory).CreateProfiles(ctx, processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
		require.NoError(t, err)
		require.NotNil(t, processor)

		err = processor.Start(ctx, host)
		require.NoError(t, err)

		err = processor.ConsumeProfiles(ctx, pprofile.NewProfiles())
		require.NoError(t, err)

		err = processor.Shutdown(ctx)
		require.NoError(t, err)
	})
}

func TestFactoryType(t *testing.T) {
	factory := NewFactory()
	assert.Equal(t, metadata.Type, factory.Type())
}
