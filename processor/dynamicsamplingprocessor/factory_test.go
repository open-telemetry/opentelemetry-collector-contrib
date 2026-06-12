// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package dynamicsamplingprocessor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/dynamicsamplingprocessor/internal/metadata"
)

func TestFactory_Type(t *testing.T) {
	f := NewFactory()
	assert.Equal(t, metadata.Type, f.Type())
}

func TestFactory_DefaultConfig(t *testing.T) {
	cfg := NewFactory().CreateDefaultConfig().(*Config)
	assert.Equal(t, 30*time.Second, cfg.TraceTimeout)
	assert.Equal(t, 2*time.Second, cfg.DecisionDelay)
	assert.Equal(t, 50_000, cfg.NumTraces)
	assert.Equal(t, 10_000, cfg.DecisionCache.SampledCacheSize)
	assert.Equal(t, 10_000, cfg.DecisionCache.NonSampledCacheSize)
	assert.Empty(t, cfg.Rules)
}

func TestFactory_CreateTracesProcessor(t *testing.T) {
	f := NewFactory()
	cfg := f.CreateDefaultConfig().(*Config)
	cfg.Rules = []RuleConfig{
		{Name: "default", Sampler: SamplerConfig{Type: AlwaysSample}},
	}

	proc, err := f.CreateTraces(t.Context(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, proc)
	assert.True(t, proc.Capabilities().MutatesData)
}
