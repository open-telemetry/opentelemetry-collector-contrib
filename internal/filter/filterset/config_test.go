// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterset

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset/regexp"
)

func readTestdataConfigYamls(t *testing.T, filename string) map[string]*Config {
	testFile := filepath.Join("testdata", filename)
	v, err := confmaptest.LoadConf(testFile)
	require.NoError(t, err)

	cfgs := map[string]*Config{}
	require.NoErrorf(t, v.Unmarshal(&cfgs, confmap.WithErrorUnused()), "unable to unmarshal yaml from file %v", testFile)
	return cfgs
}

func TestConfig(t *testing.T) {
	actualConfigs := readTestdataConfigYamls(t, "config.yaml")
	expectedConfigs := map[string]*Config{
		"regexp/default": {
			MatchType: Regexp,
		},
		"regexp/emptyoptions": {
			MatchType: Regexp,
		},
		"regexp/withoptions": {
			MatchType: Regexp,
			RegexpConfig: &regexp.Config{
				CacheEnabled:       false,
				CacheMaxNumEntries: 10,
			},
		},
		"strict/default": {
			MatchType: Strict,
		},
	}

	for testName, actualCfg := range actualConfigs {
		t.Run(testName, func(t *testing.T) {
			expCfg, ok := expectedConfigs[testName]
			assert.True(t, ok)
			assert.Equal(t, expCfg, actualCfg)

			fs, err := CreateFilterSet([]string{}, actualCfg)
			assert.NoError(t, err)
			assert.NotNil(t, fs)
		})
	}
}

func TestConfigInvalid(t *testing.T) {
	actualConfigs := readTestdataConfigYamls(t, "config_invalid.yaml")
	expectedConfigs := map[string]*Config{
		"invalid/matchtype": {
			MatchType: "invalid",
		},
	}

	for testName, actualCfg := range actualConfigs {
		t.Run(testName, func(t *testing.T) {
			expCfg, ok := expectedConfigs[testName]
			assert.True(t, ok)
			assert.Equal(t, expCfg, actualCfg)

			fs, err := CreateFilterSet([]string{}, actualCfg)
			assert.NotNil(t, err)
			assert.Nil(t, fs)
		})
	}
}
