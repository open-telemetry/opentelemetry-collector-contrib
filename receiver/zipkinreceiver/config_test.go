// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package zipkinreceiver

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/featuregate"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver/internal/metadata"
)

func TestValidateConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id             component.ID
		disallowInline bool
		expected       component.Config
		wantErr        string
	}{
		{
			id:       component.NewID(metadata.Type),
			expected: &Config{},
		},
		{
			id: component.NewIDWithName(metadata.Type, "customname"),
			expected: &Config{
				Protocols: ProtocolTypes{
					HTTP: confighttp.ServerConfig{
						Endpoint: "localhost:8765",
					},
				},
				ParseStringTags: false,
			},
		},
		{
			id:             component.NewIDWithName(metadata.Type, "customname"),
			disallowInline: true,
			expected: &Config{
				Protocols: ProtocolTypes{
					HTTP: confighttp.ServerConfig{
						Endpoint: "localhost:8765",
					},
				},
				ParseStringTags: false,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "protocols"),
			expected: &Config{
				Protocols: ProtocolTypes{
					HTTP: confighttp.ServerConfig{
						Endpoint: "localhost:8765",
					},
				},
				ParseStringTags: false,
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "protocols"),
			expected: &Config{
				Protocols: ProtocolTypes{
					HTTP: confighttp.ServerConfig{
						Endpoint: "localhost:8765",
					},
				},
				ParseStringTags: false,
			},
			disallowInline: true,
		},
		// {
		// 	id: component.NewIDWithName(metadata.Type, "parse_strings"),
		// 	expected: &Config{
		// 		ParseStringTags: true,
		// 	},
		// },
		// {
		// 	id: component.NewIDWithName(metadata.Type, "parse_strings"),
		// 	expected: &Config{
		// 		ParseStringTags: true,
		// 	},
		// 	disallowInline: true,
		// },
		{
			id:             component.NewIDWithName(metadata.Type, "deprecated"),
			disallowInline: true,
			wantErr:        "the server config setup is disabled, please use protocols::http or enable it by setting zipkinreceiver.httpDefaultProtocol.disallow feaure gate to false",
		},
		{
			id:             component.NewIDWithName(metadata.Type, "deprecated"),
			disallowInline: false,
			wantErr:        "cannot use protocols::http together with default server config setup",
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			if tt.disallowInline {
				require.NoError(t, featuregate.GlobalRegistry().Set(disallowHTTPDefaultProtocol.ID(), true))
				t.Cleanup(func() {
					require.NoError(t, featuregate.GlobalRegistry().Set(disallowHTTPDefaultProtocol.ID(), false))
				})
			}
			cfg := &Config{}

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			if tt.wantErr != "" {
				assert.Equal(t, tt.wantErr, xconfmap.Validate(cfg).Error())
			} else {
				assert.NoError(t, xconfmap.Validate(cfg))
				assert.Equal(t, tt.expected, cfg)
			}
		})
	}
}
