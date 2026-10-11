// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlecloudpubsubpushreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/googlecloudpubsubpushreceiver/internal/metadata"
)

func TestCreateDefaultConfigRoundTrip(t *testing.T) {
	cfg := createDefaultConfig()
	cm := confmap.New()
	require.NoError(t, cm.Marshal(cfg))
	// Unmarshal back into cfg itself so that unexported bookkeeping fields
	// (e.g. confighttp.ServerConfig's deprecation warnings, which are only
	// populated by Unmarshal) settle into the same state as roundTrip below.
	require.NoError(t, cm.Unmarshal(cfg))

	roundTrip := createDefaultConfig()
	require.NoError(t, cm.Unmarshal(roundTrip))
	require.Equal(t, cfg, roundTrip)
}

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
				cfg.Encoding = new(component.MustNewID("test"))
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

			err = confmap.Validate(cfg)
			if tt.expectedErr != "" {
				require.ErrorContains(t, err, tt.expectedErr)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expected, cfg)
		})
	}
}
