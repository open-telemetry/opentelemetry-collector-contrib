// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package filterprocessor

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configtest"
	"go.opentelemetry.io/collector/consumer/consumertest"
)

func TestType(t *testing.T) {
	factory := NewFactory()
	pType := factory.Type()

	assert.Equal(t, pType, config.Type("filter"))
}

func TestCreateDefaultConfig(t *testing.T) {
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	assert.Equal(t, cfg, &Config{
		ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
	})
	assert.NoError(t, configtest.CheckConfigStruct(cfg))
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
				require.NoError(t, config.UnmarshalProcessor(sub, cfg))

				tp, tErr := factory.CreateTracesProcessor(
					context.Background(),
					componenttest.NewNopProcessorCreateSettings(),
					cfg, consumertest.NewNop(),
				)
				mp, mErr := factory.CreateMetricsProcessor(
					context.Background(),
					componenttest.NewNopProcessorCreateSettings(),
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
