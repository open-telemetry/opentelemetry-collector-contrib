// Copyright 2020 OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package groupbyattrsprocessor

import (
	"path/filepath"
	"testing"

	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
)

func TestLoadingConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id       config.ComponentID
		expected config.Processor
	}{
		{
			id: config.NewComponentIDWithName(typeStr, "grouping"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				GroupByKeys:       []string{"key1", "key2"},
			},
		},
		{
			id: config.NewComponentIDWithName(typeStr, "compaction"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID(typeStr)),
				GroupByKeys:       []string{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
			require.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, config.UnmarshalProcessor(sub, cfg))

			assert.NoError(t, cfg.Validate())
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
