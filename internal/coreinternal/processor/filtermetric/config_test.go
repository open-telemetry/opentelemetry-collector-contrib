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

package filtermetric

import (
	"path/filepath"
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterconfig"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configtest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/processor/filterset/regexp"
)

var (
	// regexpNameMatches matches the metrics names specified in testdata/config.yaml
	regexpNameMatches = []string{
		"prefix/.*",
		".*contains.*",
		".*_suffix",
		"full_name_match",
	}

	strictNameMatches = []string{
		"exact_string_match",
	}
)

func TestConfig(t *testing.T) {
	testFile := filepath.Join("testdata", "config.yaml")
	v, err := configtest.LoadConfigMap(testFile)
	require.NoError(t, err)
	testYamls := map[string]filterconfig.MatchProperties{}
	require.NoErrorf(t, v.UnmarshalExact(&testYamls), "unable to unmarshal yaml from file %v", testFile)

	tests := []struct {
		name   string
		expCfg *filterconfig.MatchProperties
	}{
		{
			name: "config/regexp",
			expCfg: &filterconfig.MatchProperties{
				Config:      filterset.Config{MatchType: filterset.Regexp},
				MetricNames: regexpNameMatches,
			},
		}, {
			name: "config/regexpoptions",
			expCfg: &filterconfig.MatchProperties{
				Config: filterset.Config{
					MatchType: filterset.Regexp,
					RegexpConfig: &regexp.Config{
						CacheEnabled:       true,
						CacheMaxNumEntries: 5,
					},
				},
				MetricNames: regexpNameMatches,
			},
		}, {
			name: "config/strict",
			expCfg: &filterconfig.MatchProperties{
				Config:      filterset.Config{MatchType: filterset.Strict},
				MetricNames: strictNameMatches,
			},
		},
		{
			name: "config/emptyproperties",
			expCfg: &filterconfig.MatchProperties{
				Config: filterset.Config{MatchType: filterset.Regexp},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := testYamls[test.name]
			assert.Equal(t, *test.expCfg, cfg)

			matcher, err := NewMatcher(&cfg)
			assert.NotNil(t, matcher)
			assert.NoError(t, err)
		})
	}
}
