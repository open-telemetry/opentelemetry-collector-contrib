// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package githubactionsreceiver

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubactionsreceiver/internal/metadata"
)

// only one validate check so far
func TestValidateConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc   string
		expect error
		conf   Config
	}{
		{
			desc:   "Missing valid endpoint",
			expect: errMissingEndpointFromConfig,
			conf: Config{
				HTTPServerSettings: confighttp.HTTPServerSettings{
					Endpoint: "",
				},
			},
		},
		{
			desc:   "Valid Secret",
			expect: nil,
			conf: Config{
				HTTPServerSettings: confighttp.HTTPServerSettings{
					Endpoint: "localhost:8080",
				},
				Secret: "mysecret",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.conf.Validate()
			if test.expect == nil {
				require.NoError(t, err)
				require.Equal(t, "mysecret", test.conf.Secret)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), test.expect.Error())
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	// LoadConf includes the TypeStr which NewFactory does not set
	id := component.NewIDWithName(metadata.Type, "valid_config")
	cmNoStr, err := cm.Sub(id.String())
	require.NoError(t, err)

	expect := &Config{
		HTTPServerSettings: confighttp.HTTPServerSettings{
			Endpoint: "localhost:8080",
		},
		Path:   "/events",
		Secret: "mysecret",
	}

	// create expected config
	factory := NewFactory()
	conf := factory.CreateDefaultConfig()
	require.NoError(t, component.UnmarshalConfig(cmNoStr, conf))
	require.NoError(t, component.ValidateConfig(conf))

	require.Equal(t, expect, conf)
}
