// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourcedetectionprocessor

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/collector/processor/xprocessor"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
	assert.NotNil(t, cfg)
}

func TestCreateProcessor(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	tp, err := factory.CreateTraces(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, tp)

	mp, err := factory.CreateMetrics(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, mp)

	lp, err := factory.CreateLogs(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
	assert.NoError(t, err)
	assert.NotNil(t, lp)

	pp, err := factory.(xprocessor.Factory).CreateProfiles(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
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
				context.Background(),
				processortest.NewNopSettings(),
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

	tp, err := factory.CreateTraces(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
	assert.Error(t, err)
	assert.Nil(t, tp)

	mp, err := factory.CreateMetrics(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
	assert.Error(t, err)
	assert.Nil(t, mp)

	lp, err := factory.CreateLogs(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
	assert.Error(t, err)
	assert.Nil(t, lp)

	pp, err := factory.(xprocessor.Factory).CreateProfiles(context.Background(), processortest.NewNopSettings(), cfg, consumertest.NewNop())
	assert.Error(t, err)
	assert.Nil(t, pp)
}
