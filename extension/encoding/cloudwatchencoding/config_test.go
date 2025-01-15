// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cloudwatchencoding

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/extension/encoding/cloudwatchencodingextension/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	defaultCfg := createDefaultConfig().(*Config)

	tests := []struct {
		id          component.ID
		expected    component.Config
		expectedErr error
	}{
		{
			id:       component.NewID(metadata.Type),
			expected: defaultCfg,
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_encoding"),
			expectedErr: errors.New("unknown content encoding \"invalid\""),
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

			if tt.expectedErr != nil {
				err = component.ValidateConfig(cfg)
				require.Equal(t, tt.expectedErr, err)
				return
			}

			require.NoError(t, component.ValidateConfig(cfg))
			require.Equal(t, tt.expected, cfg)
		})
	}
}
