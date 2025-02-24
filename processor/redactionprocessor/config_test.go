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
				HashFunction:       MD5,
				BlockedKeyPatterns: []string{".*token.*", ".*api_key.*"},
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
		cfg      Config
		expected error
	}{
		{
			name: "valid",
			cfg: Config{
				HashFunction: MD5,
			},
		},
		{
			name: "empty",
			cfg:  Config{},
		},
		{
			name: "invalid",
			cfg: Config{
				HashFunction: "hash",
			},
			expected: errors.New("unsupported hash function: 'hash'. Supported functions are: 'md5', 'sha1', 'sha3'"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.cfg.Validate())
		})
	}
}
