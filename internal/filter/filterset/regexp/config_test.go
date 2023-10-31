// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package regexp

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func TestConfig(t *testing.T) {
	testFile := filepath.Join("testdata", "config.yaml")
	v, err := confmaptest.LoadConf(testFile)
	require.NoError(t, err)

	actualConfigs := map[string]*Config{}
	require.NoErrorf(t, v.Unmarshal(&actualConfigs, confmap.WithErrorUnused()),
		"unable to unmarshal yaml from file %v", testFile)

	expectedConfigs := map[string]*Config{
		"regexp/default": {},
		"regexp/cachedisabledwithsize": {
			CacheEnabled:       false,
			CacheMaxNumEntries: 10,
		},
		"regexp/cacheenablednosize": {
			CacheEnabled: true,
		},
	}

	for testName, actualCfg := range actualConfigs {
		t.Run(testName, func(t *testing.T) {
			expCfg, ok := expectedConfigs[testName]
			assert.True(t, ok)
			assert.Equal(t, expCfg, actualCfg)

			fs, err := NewFilterSet([]string{}, actualCfg)
			assert.NoError(t, err)
			assert.NotNil(t, fs)
		})
	}
}
