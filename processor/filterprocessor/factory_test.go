// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterprocessor

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/processor/processortest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func TestType(t *testing.T) {
	factory := NewFactory()
	pType := factory.Type()

	assert.Equal(t, pType, component.Type("filter"))
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.Equal(t, cfg, &Config{
		ErrorMode: ottl.PropagateError,
	})
	assert.NoError(t, componenttest.CheckConfigStruct(cfg))
}

func TestCreateProcessors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		configName string
		succeed    bool
	}{
		{
			configName: "config_regexp.yaml",
			succeed:    true,
		}, {
			configName: "config_strict.yaml",
			succeed:    true,
		}, {
			configName: "config_invalid.yaml",
			succeed:    false,
		}, {
			configName: "config_logs_strict.yaml",
			succeed:    true,
		}, {
			configName: "config_logs_regexp.yaml",
			succeed:    true,
		}, {
			configName: "config_logs_record_attributes_strict.yaml",
			succeed:    true,
		}, {
			configName: "config_logs_record_attributes_regexp.yaml",
			succeed:    true,
		}, {
			configName: "config_traces.yaml",
			succeed:    true,
		}, {
			configName: "config_traces_invalid.yaml",
			succeed:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.configName, func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", tt.configName))
			require.NoError(t, err)

			for k := range cm.ToStringMap() {
				// Check if all processor variations that are defined in test config can be actually created
				factory := NewFactory()
				cfg := factory.CreateDefaultConfig()

				sub, err := cm.Sub(k)
				require.NoError(t, err)
				require.NoError(t, component.UnmarshalConfig(sub, cfg))

				tp, tErr := factory.CreateTracesProcessor(
					context.Background(),
					processortest.NewNopCreateSettings(),
					cfg, consumertest.NewNop(),
				)
				mp, mErr := factory.CreateMetricsProcessor(
					context.Background(),
					processortest.NewNopCreateSettings(),
					cfg,
					consumertest.NewNop(),
				)
				if strings.Contains(tt.configName, "traces") {
					assert.Equal(t, tt.succeed, tp != nil)
					assert.Equal(t, tt.succeed, tErr == nil)

					assert.NotNil(t, mp)
					assert.Nil(t, mErr)
				} else {
					// Should not break configs with no trace data
					assert.NotNil(t, tp)
					assert.Nil(t, tErr)

					assert.Equal(t, tt.succeed, mp != nil)
					assert.Equal(t, tt.succeed, mErr == nil)
				}
			}
		})
	}
}
