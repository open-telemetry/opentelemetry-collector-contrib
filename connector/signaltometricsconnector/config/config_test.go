// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/internal/metadata"
)

func TestConfig(t *testing.T) {
	for _, tc := range []struct {
		path      string // relative to `testdata/configs` directory
		expected  *Config
		errorMsgs []string // all error message are checked
	}{
		{
			path:      "empty",
			errorMsgs: []string{"no configuration provided"},
		},
	} {
		t.Run(tc.path, func(t *testing.T) {
			dir := filepath.Join("..", "testdata", "configs")
			cfg := &Config{}
			cm, err := confmaptest.LoadConf(filepath.Join(dir, tc.path+".yaml"))
			require.NoError(t, err)

			sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(&cfg))

			err = component.ValidateConfig(cfg)
			if len(tc.errorMsgs) > 0 {
				for _, errMsg := range tc.errorMsgs {
					assert.ErrorContains(t, err, errMsg)
				}
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tc.expected, cfg)
		})
	}
}
