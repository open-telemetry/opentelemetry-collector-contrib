// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkhecreceiver

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
	translator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/splunk"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkhecreceiver/internal/metadata"
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
			id:       component.NewID(metadata.Type),
			expected: createDefaultConfig(),
		},
		{
			id: component.NewIDWithName(metadata.Type, "allsettings"),
			expected: &Config{
				ServerConfig: confighttp.ServerConfig{
					Endpoint: "localhost:8088",
				},
				AccessTokenPassthroughConfig: splunk.AccessTokenPassthroughConfig{
					AccessTokenPassthrough: true,
				},
				RawPath:    "/foo",
				Splitting:  SplittingStrategyLine,
				HealthPath: "/bar",
				Ack:        Ack{Path: "/services/collector/ack"},
				HecToOtelAttrs: translator.HecToOtelAttrs{
					Source:     "file.name",
					SourceType: "foobar",
					Index:      "myindex",
					Host:       "myhostfield",
				},
			},
		},
		{
			id: component.NewIDWithName(metadata.Type, "tls"),
			expected: &Config{
				ServerConfig: confighttp.ServerConfig{
					Endpoint: "localhost:8088",
					TLS: configoptional.Some(configtls.ServerConfig{
						Config: configtls.Config{
							CertFile: "/test.crt",
							KeyFile:  "/test.key",
						},
					}),
				},
				AccessTokenPassthroughConfig: splunk.AccessTokenPassthroughConfig{
					AccessTokenPassthrough: false,
				},
				RawPath:    "/services/collector/raw",
				Splitting:  SplittingStrategyLine,
				HealthPath: "/services/collector/health",
				Ack:        Ack{Path: "/services/collector/ack"},
				HecToOtelAttrs: translator.HecToOtelAttrs{
					Source:     "com.splunk.source",
					SourceType: "com.splunk.sourcetype",
					Index:      "com.splunk.index",
					Host:       "host.name",
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
			require.NoError(t, sub.Unmarshal(cfg))

			assert.NoError(t, xconfmap.Validate(cfg))
			assert.Equal(t, tt.expected, cfg)
		})
	}
}
