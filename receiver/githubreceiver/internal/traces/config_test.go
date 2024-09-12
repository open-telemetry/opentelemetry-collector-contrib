// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package traces // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/githubreceiver/internal/traces"

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/webhookeventreceiver"
)

// only one validate check so far
func TestValidateConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc   string
		expect error
		conf   Config
	}{
		// Endpoint is allowed to be empty - it is the unconfigured state. The tracing functionality is
		// disabled if the endpoint is empty.
		//{
		//	desc:   "Missing valid endpoint",
		//	expect: errMissingEndpointFromConfig,
		//	conf: Config{
		//		WebhookReceiver: webhookeventreceiver.Config{
		//			ServerConfig: confighttp.ServerConfig{
		//				Endpoint: "",
		//			},
		//		},
		//	},
		//},
		{
			desc:   "Valid Secret",
			expect: nil,
			conf: Config{
				WebhookReceiver: webhookeventreceiver.Config{
					ServerConfig: confighttp.ServerConfig{
						Endpoint: "localhost:8080",
					},
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

	// create expected config
	expect := &Config{
		WebhookReceiver: webhookeventreceiver.Config{
			ServerConfig: confighttp.ServerConfig{
				Endpoint: "localhost:999",
			},
			Path: "/ghaevents-for-real",
		},
		Secret: "my real secret",
	}

	conf := &Config{}

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	require.NoError(t, cm.Unmarshal(conf))
	require.NoError(t, component.ValidateConfig(conf))

	require.Equal(t, expect, conf)
}
