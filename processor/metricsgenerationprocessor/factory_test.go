// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package metricsgenerationprocessor

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
	"go.opentelemetry.io/collector/processor/processortest"
)

func TestType(t *testing.T) {
	factory := NewFactory()
	pType := factory.Type()
	assert.Equal(t, pType, component.Type("experimental_metricsgeneration"))
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.Equal(t, cfg, &Config{})
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
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			tp, tErr := factory.CreateTracesProcessor(
				context.Background(),
				processortest.NewNopCreateSettings(),
				cfg,
				consumertest.NewNop())
			// Not implemented error
			assert.Error(t, tErr)
			assert.Nil(t, tp)

			mp, mErr := factory.CreateMetricsProcessor(
				context.Background(),
				processortest.NewNopCreateSettings(),
				cfg,
				consumertest.NewNop())
			assert.NotNil(t, mp)
			assert.NoError(t, mErr)
		})
	}
}
