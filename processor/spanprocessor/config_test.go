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
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
)

func TestLoadingConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName("span", "custom"),
			expected: &Config{
				Rename: Name{
					FromAttributes: []string{"db.svc", "operation", "id"},
					Separator:      "::",
				},
			},
		},
		{
			id: component.NewIDWithName("span", "no-separator"),
			expected: &Config{
				Rename: Name{
					FromAttributes: []string{"db.svc", "operation", "id"},
					Separator:      "",
				},
			},
		},
		{
			id: component.NewIDWithName("span", "to_attributes"),
			expected: &Config{
				Rename: Name{
					ToAttributes: &ToAttributes{
						Rules: []string{`^\/api\/v1\/document\/(?P<documentId>.*)\/update$`},
					},
				},
			},
		},
		{
			id: component.NewIDWithName("span", "includeexclude"),
			expected: &Config{
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
			id: component.NewIDWithName("span", "set_status_err"),
			expected: &Config{
				SetStatus: &Status{
					Code:        "Error",
					Description: "some additional error description",
				},
			},
		},
		{
			id: component.NewIDWithName("span", "set_status_ok"),
			expected: &Config{
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
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func createMatchConfig(matchType filterset.MatchType) *filterset.Config {
	return &filterset.Config{
		MatchType: matchType,
	}
}
