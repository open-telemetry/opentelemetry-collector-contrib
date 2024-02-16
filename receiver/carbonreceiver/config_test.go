// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package carbonreceiver

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/carbonreceiver/protocol"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	tests := []struct {
		id       component.ID
		expected component.Config
	}{
		{
			id:       component.NewIDWithName(metadata.Type, ""),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "receiver_settings"),
			expected: &Config{
				NetAddr: confignet.NetAddr{
					Endpoint:  "localhost:8080",
					Transport: "udp",
				},
				TCPIdleTimeout: 5 * time.Second,
				Parser: &protocol.Config{
					Type:   "plaintext",
					Config: &protocol.PlaintextConfig{},
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "regex"),
			expected: &Config{
				NetAddr: confignet.NetAddr{
					Endpoint:  "localhost:2003",
					Transport: "tcp",
				},
				TCPIdleTimeout: 30 * time.Second,
				Parser: &protocol.Config{
					Type: "regex",
					Config: &protocol.RegexParserConfig{
						Rules: []*protocol.RegexRule{
							{
								Regexp:     `(?P<key_base>test)\.env(?P<key_env>[^.]*)\.(?P<key_host>[^.]*)`,
								NamePrefix: "name-prefix",
								Labels: map[string]string{
									"dot.key": "dot.value",
									"key":     "value",
								},
								MetricType: "cumulative",
							},
							{
								Regexp: `(?P<key_just>test)\.(?P<key_match>.*)`,
							},
						},
						MetricNameSeparator: "_",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, component.UnmarshalConfig(sub, cfg))

			assert.NoError(t, component.ValidateConfig(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}

func TestConfigValidate(t *testing.T) {
	cfg := &Config{
		NetAddr: confignet.NetAddr{
			Endpoint:  "localhost:2003",
			Transport: "tcp",
		},
		TCPIdleTimeout: -1 * time.Second,
		Parser: &protocol.Config{
			Type:   "plaintext",
			Config: &protocol.PlaintextConfig{},
		},
	}
	assert.Error(t, cfg.Validate())
}
