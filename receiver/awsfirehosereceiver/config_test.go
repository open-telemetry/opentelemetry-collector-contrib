// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsfirehosereceiver

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awsfirehosereceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	for _, configType := range []string{
		"cwmetrics", "cwlogs", "otlp_v1",
	} {
		t.Run(configType, func(t *testing.T) {
			fileName := configType + "_config.yaml"
			cm, err := confmaptest.LoadConf(filepath.Join("testdata", fileName))
			require.NoError(t, err)

			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			err = xconfmap.Validate(cfg)
			assert.NoError(t, err)
			serverConfig := confighttp.NewDefaultServerConfig()
			// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
			serverConfig.WriteTimeout = 0
			serverConfig.ReadHeaderTimeout = 0
			serverConfig.IdleTimeout = 0
			serverConfig.KeepAlivesEnabled = false
			serverConfig.NetAddr = confignet.AddrConfig{
				Transport: "tcp",
				Endpoint:  "0.0.0.0:4433",
			}
			serverConfig.TLS = configoptional.Some(configtls.ServerConfig{
				Config: configtls.Config{
					CertFile: "server.crt",
					KeyFile:  "server.key",
				},
			})
			require.Equal(t, &Config{
				RecordType:   configType,
				AccessKey:    "some_access_key",
				ServerConfig: serverConfig,
			}, cfg)
		})
	}
}

func TestLoadConfigInvalid(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "invalid_config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	err = xconfmap.Validate(cfg)
	assert.ErrorIs(t, err, errRecordTypeEncodingSet)
}

func TestValidate(t *testing.T) {
	testCases := map[string]struct {
		cfg     *Config
		wantErr string
	}{
		"negative record_decompressed_size_limit": {
			cfg: &Config{
				ServerConfig: confighttp.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint: "localhost:8443",
					},
				},
				RecordDecompressedSizeLimit: -1,
			},
			wantErr: "record_decompressed_size_limit must be non-negative",
		},
		"negative request_decompressed_size_limit": {
			cfg: &Config{
				ServerConfig: confighttp.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint: "localhost:8443",
					},
				},
				RequestDecompressedSizeLimit: -1,
			},
			wantErr: "request_decompressed_size_limit must be non-negative",
		},
		"valid configuration": {
			cfg: &Config{
				ServerConfig: confighttp.ServerConfig{
					NetAddr: confignet.AddrConfig{
						Endpoint: "localhost:8443",
					},
				},
				RecordDecompressedSizeLimit:  10,
				RequestDecompressedSizeLimit: 20,
			},
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
