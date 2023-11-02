// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package collectdreceiver

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/collectdreceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		id       component.ID
		expected component.Config
		wantErr  error
	}{
		{
			id:       component.NewIDWithName(metadata.Type, ""),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "one"),
			expected: &Config{
				HTTPServerSettings: confighttp.HTTPServerSettings{
					Endpoint: "localhost:12345",
				},
				Timeout:          50 * time.Second,
				AttributesPrefix: "dap_",
				Encoding:         "command",
			},
			wantErr: errors.New("CollectD only support JSON encoding format. command is not supported"),
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
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			if tt.wantErr == nil {
				assert.NoError(t, component.ValidateConfig(cfg))
			} else {
				assert.Equal(t, tt.wantErr, component.ValidateConfig(cfg))
			}
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
