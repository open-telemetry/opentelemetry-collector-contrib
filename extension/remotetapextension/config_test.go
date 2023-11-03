// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package remotetapextension

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap/confmaptest"
)

func Test_Config_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *Config
		wantErr error
	}{
		{
			"valid",
			func() *Config {
				return &Config{
					HTTPServerSettings: confighttp.HTTPServerSettings{
						Endpoint: "localhost:5678",
					},
					Taps: []TapInfo{
						{
							Name:     "foo",
							Endpoint: "localhost:8082",
						},
					},
				}
			}(),
			nil,
		},
		{
			"no taps",
			func() *Config {
				return &Config{
					HTTPServerSettings: confighttp.HTTPServerSettings{
						Endpoint: "localhost:5678",
					},
					Taps: []TapInfo{},
				}
			}(),
			errors.New("no taps defined"),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.cfg.Validate()
			if test.wantErr == nil {
				require.NoError(t, err)
			} else {
				require.Equal(t, test.wantErr, err)
			}
		})
	}
}

func Test_LoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	sub, err := cm.Sub("remotetap")
	require.NoError(t, err)
	var cfg Config
	require.NoError(t, component.UnmarshalConfig(sub, &cfg))
	require.NoError(t, cfg.Validate())
}
