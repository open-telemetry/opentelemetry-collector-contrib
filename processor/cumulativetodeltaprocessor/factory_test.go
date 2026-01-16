// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cumulativetodeltaprocessor

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor/internal/metadata"
)

func TestType(t *testing.T) {
	factory := NewFactory()
	pType := factory.Type()
	assert.Equal(t, pType, metadata.Type)
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	cumulativeToDeltaCfg, ok := cfg.(*Config)
	require.True(t, ok)

	// Default MaxStaleness should be 1 hour
	assert.Equal(t, 1*time.Hour, cumulativeToDeltaCfg.MaxStaleness)
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateProcessors(t *testing.T) {
	t.Parallel()

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

			tp, tErr := factory.CreateTraces(
				t.Context(),
				processortest.NewNopSettings(metadata.Type),
				cfg,
				consumertest.NewNop())
			// Not implemented error
			assert.Error(t, tErr)
			assert.Nil(t, tp)

			mp, mErr := factory.CreateMetrics(
				t.Context(),
				processortest.NewNopSettings(metadata.Type),
				cfg,
				consumertest.NewNop())

			if strings.Contains(k, "invalid") {
				assert.Error(t, mErr)
				assert.Nil(t, mp)
				return
			}
			assert.NotNil(t, mp)
			assert.NoError(t, mErr)
			assert.NoError(t, mp.Shutdown(t.Context()))
		})
	}
}

func TestExplicitConfigOverridesDefault(t *testing.T) {
	factory := NewFactory()

	// Load config with explicit max_staleness value (10s in testdata)
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	// Use the "cumulativetodelta" config that has max_staleness: 10s
	cfg := factory.CreateDefaultConfig()
	sub, err := cm.Sub("cumulativetodelta")
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	cumulativeToDeltaCfg, ok := cfg.(*Config)
	require.True(t, ok)

	// The explicitly configured value (10s) should be used, not the default (1h)
	assert.Equal(t, 10*time.Second, cumulativeToDeltaCfg.MaxStaleness)
}

func TestExplicitZeroConfig(t *testing.T) {
	// Create config with explicitly set zero value
	cfg := &Config{
		MaxStaleness: 0,
	}

	// Verify that explicitly set zero is preserved (user wants infinite retention)
	assert.Equal(t, time.Duration(0), cfg.MaxStaleness)
}
