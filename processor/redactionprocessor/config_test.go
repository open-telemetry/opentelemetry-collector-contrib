// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redactionprocessor

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/redactionprocessor/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: &Config{
				AllowAllKeys:       false,
				AllowedKeys:        []string{"description", "group", "id", "name"},
				IgnoredKeys:        []string{"safe_attribute"},
				BlockedValues:      []string{"4[0-9]{12}(?:[0-9]{3})?", "(5[1-5][0-9]{14})"},
				BlockedKeyPatterns: []string{".*token.*", ".*api_key.*"},
				HashFunction:       MD5,
				AllowedValues:      []string{".+@mycompany.com"},
				Summary:            debug,
			},
		},
		{
			id:       component.NewIDWithName(metadata.Type, "empty"),
			expected: createDefaultConfig(),
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
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name     string
		hash     HashFunction
		expected error
	}{
		{
			name: "valid",
			hash: MD5,
		},
		{
			name: "empty",
			hash: None,
		},
		{
			name:     "invalid",
			hash:     "hash",
			expected: errors.New("unknown HashFunction hash, allowed functions are sha1, sha3 and md5"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var h HashFunction
			err := h.UnmarshalText([]byte(tt.hash))
			if tt.expected != nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.hash, h)
			}
		})
	}
}
