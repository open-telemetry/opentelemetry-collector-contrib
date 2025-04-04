// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package bearertokenauthextension

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/bearertokenauthextension/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id          component.ID
		expected    component.Config
		expectedErr bool
	}{
		{
			id:          component.NewID(metadata.Type),
			expectedErr: true,
		},
		{
			id: component.NewIDWithName(metadata.Type, "sometoken"),
			expected: &Config{
				Header:      defaultHeader,
				Scheme:      defaultScheme,
				BearerToken: "sometoken",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "withscheme"),
			expected: &Config{
				Header:      defaultHeader,
				Scheme:      "MyScheme",
				BearerToken: "my-token",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "multipletokens"),
			expected: &Config{
				Header: defaultHeader,
				Scheme: "Bearer",
				Tokens: []configopaque.String{"token1", "thistokenalsoworks"},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "withfilename"),
			expected: &Config{
				Header:   defaultHeader,
				Scheme:   "Bearer",
				Filename: "file-containing.token",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "both"),
			expected: &Config{
				Header:      defaultHeader,
				Scheme:      "Bearer",
				BearerToken: "ignoredtoken",
				Filename:    "file-containing.token",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "tokensandtoken"),
			expected: &Config{
				Header:      defaultHeader,
				Scheme:      "Bearer",
				BearerToken: "sometoken",
				Tokens:      []configopaque.String{"token1", "thistokenalsoworks"},
			},
			expectedErr: true,
		},
		{
			id: component.NewIDWithName(metadata.Type, "withtokensandfilename"),
			expected: &Config{
				Header:   defaultHeader,
				Scheme:   "Bearer",
				Tokens:   []configopaque.String{"ignoredtoken1", "ignoredtoken2"},
				Filename: "file-containing.token",
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "withheader"),
			expected: &Config{
				Header:      "X-Custom-Authorization",
				Scheme:      "",
				BearerToken: "my-token",
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
			require.NoError(t, sub.Unmarshal(cfg))
			if tt.expectedErr {
				assert.Error(t, xconfmap.Validate(cfg))
				return
			}
			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
