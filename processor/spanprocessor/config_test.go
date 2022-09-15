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

package spanprocessor

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset"
)

func TestLoadingConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id       config.ComponentID
		expected config.Processor
	}{
		{
			id: config.NewComponentIDWithName("span", "custom"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID("span")),
				Rename: Name{
					FromAttributes: []string{"db.svc", "operation", "id"},
					Separator:      "::",
				},
			},
		},
		{
			id: config.NewComponentIDWithName("span", "no-separator"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID("span")),
				Rename: Name{
					FromAttributes: []string{"db.svc", "operation", "id"},
					Separator:      "",
				},
			},
		},
		{
			id: config.NewComponentIDWithName("span", "to_attributes"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID("span")),
				Rename: Name{
					ToAttributes: &ToAttributes{
						Rules: []string{`^\/api\/v1\/document\/(?P<documentId>.*)\/update$`},
					},
				},
			},
		},
		{
			id: config.NewComponentIDWithName("span", "includeexclude"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID("span")),
				MatchConfig: filterconfig.MatchConfig{
					Include: &filterconfig.MatchProperties{
						Config:    *createMatchConfig(filterset.Regexp),
						Services:  []string{`banks`},
						SpanNames: []string{"^(.*?)/(.*?)$"},
					},
					Exclude: &filterconfig.MatchProperties{
						Config:    *createMatchConfig(filterset.Strict),
						SpanNames: []string{`donot/change`},
					},
				},
				Rename: Name{
					ToAttributes: &ToAttributes{
						Rules: []string{`(?P<operation_website>.*?)$`},
					},
				},
			},
		},
		{
			// Set name
			id: config.NewComponentIDWithName("span", "set_status_err"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID("span")),
				SetStatus: &Status{
					Code:        "Error",
					Description: "some additional error description",
				},
			},
		},
		{
			id: config.NewComponentIDWithName("span", "set_status_ok"),
			expected: &Config{
				ProcessorSettings: config.NewProcessorSettings(config.NewComponentID("span")),
				MatchConfig: filterconfig.MatchConfig{
					Include: &filterconfig.MatchProperties{
						Attributes: []filterconfig.Attribute{
							{Key: "http.status_code", Value: 400},
						},
					},
				},
				SetStatus: &Status{
					Code: "Ok",
				},
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

func createMatchConfig(matchType filterset.MatchType) *filterset.Config {
	return &filterset.Config{
		MatchType: matchType,
	}
}
