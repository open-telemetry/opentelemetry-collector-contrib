// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudpubsubpushreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubpushreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf("testdata/config.yaml")
	require.NoError(t, err)

	tests := []struct {
		id          component.ID
		expected    component.Config
		expectedErr string
	}{
		{
			id: component.NewIDWithName(metadata.Type, ""),
			expected: func() component.Config {
				cfg := createDefaultConfig().(*Config)
				cfg.Encoding = component.MustNewID("test")
				return cfg
			}(),
		},
		{
			id:          component.NewIDWithName(metadata.Type, "empty_encoding"),
			expectedErr: "encoding must be set",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "misformatted_endpoint"),
			expectedErr: "misformatted endpoint",
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.Name(), func(t *testing.T) {
			t.Parallel()

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			err = xconfmap.Validate(cfg)
			if tt.expectedErr != "" {
				require.ErrorContains(t, err, tt.expectedErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expected, cfg)
		})
	}
}
