// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cumulativetodeltaprocessor

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

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
	assert.Equal(t, &Config{}, cfg)
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
				context.Background(),
				processortest.NewNopSettings(metadata.Type),
				cfg,
				consumertest.NewNop())
			// Not implemented error
			assert.Error(t, tErr)
			assert.Nil(t, tp)

			mp, mErr := factory.CreateMetrics(
				context.Background(),
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
			assert.NoError(t, mp.Shutdown(context.Background()))
		})
	}
}
