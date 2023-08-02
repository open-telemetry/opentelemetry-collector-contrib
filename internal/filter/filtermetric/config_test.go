// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filtermetric

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset/regexp"
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

func createConfigWithRegexpOptions(filters []string, rCfg *regexp.Config) *filterconfig.MetricMatchProperties {
	cfg := createConfig(filters, filterset.Regexp)
	cfg.RegexpConfig = rCfg
	return cfg
}

func TestConfig(t *testing.T) {
	testFile := filepath.Join("testdata", "config.yaml")
	v, err := confmaptest.LoadConf(testFile)
	require.NoError(t, err)
	testYamls := map[string]filterconfig.MetricMatchProperties{}
	require.NoErrorf(t, v.Unmarshal(&testYamls, confmap.WithErrorUnused()), "unable to unmarshal yaml from file %v", testFile)

	tests := []struct {
		name   string
		expCfg *filterconfig.MetricMatchProperties
	}{
		{
			name:   "config/regexp",
			expCfg: createConfig(regexpNameMatches, filterset.Regexp),
		},
		{
			name: "config/regexpoptions",
			expCfg: createConfigWithRegexpOptions(
				regexpNameMatches,
				&regexp.Config{
					CacheEnabled:       true,
					CacheMaxNumEntries: 5,
				},
			),
		},
		{
			name:   "config/strict",
			expCfg: createConfig(strictNameMatches, filterset.Strict),
		},
		{
			name:   "config/emptyproperties",
			expCfg: createConfig(nil, filterset.Regexp),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := testYamls[test.name]
			assert.Equal(t, *test.expCfg, cfg)

			_, err := newExpr(&cfg)
			assert.NoError(t, err)
		})
	}
}
