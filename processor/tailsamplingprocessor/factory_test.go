// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tailsamplingprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/internal/metadata"
)

func TestCreateDefaultConfig(t *testing.T) {
	cfg := createDefaultConfig()
	assert.NotNil(t, cfg, "failed to create default config")
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateProcessor(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "tail_sampling_config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	params := processortest.NewNopSettings(metadata.Type)
	tp, err := factory.CreateTraces(t.Context(), params, cfg, consumertest.NewNop())
	assert.NotNil(t, tp)
	assert.NoError(t, err, "cannot create trace processor")

	// this will cause the processor to properly initialize, so that we can later shutdown and
	// have all the go routines cleanly shut down
	assert.NoError(t, tp.Start(t.Context(), componenttest.NewNopHost()))
	assert.NoError(t, tp.Shutdown(t.Context()))
}

func TestCreateProcessorRejectsInvalidSamplingStrategy(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.SamplingStrategy = "invalid"

	params := processortest.NewNopSettings(metadata.Type)
	tp, err := factory.CreateTraces(t.Context(), params, cfg, consumertest.NewNop())
	assert.Nil(t, tp)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid sampling_strategy")
}

func TestCreateProcessorAllowsSpanIngest(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.SamplingStrategy = samplingStrategySpanIngest
	cfg.PolicyCfgs = []PolicyCfg{
		{
			sharedPolicyCfg: sharedPolicyCfg{
				Name: "policy",
				Type: Probabilistic,
				ProbabilisticCfg: ProbabilisticCfg{
					SamplingPercentage: 1,
				},
			},
		},
	}

	params := processortest.NewNopSettings(metadata.Type)
	tp, err := factory.CreateTraces(t.Context(), params, cfg, consumertest.NewNop())
	assert.NotNil(t, tp)
	assert.NoError(t, err)
}

func TestCreateProcessorRejectsStatefulPolicyForSpanIngest(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig().(*Config)
	cfg.SamplingStrategy = samplingStrategySpanIngest
	cfg.PolicyCfgs = []PolicyCfg{
		{
			sharedPolicyCfg: sharedPolicyCfg{
				Name: "stateful-policy",
				Type: RateLimiting,
				RateLimitingCfg: RateLimitingCfg{
					SpansPerSecond: 10,
				},
			},
		},
	}

	params := processortest.NewNopSettings(metadata.Type)
	tp, err := factory.CreateTraces(t.Context(), params, cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, tp)
	err = tp.Start(t.Context(), componenttest.NewNopHost())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires all policies to be stateless")
	assert.Contains(t, err.Error(), "stateful-policy")
}
