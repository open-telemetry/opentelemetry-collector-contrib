// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package httpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/httpcheckreceiver"

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"
)

func TestValidate(t *testing.T) {
	clientConfigMissingEndpoint := confighttp.NewDefaultClientConfig()

	clientConfigInvalidEndpoint := confighttp.NewDefaultClientConfig()
	clientConfigInvalidEndpoint.Endpoint = "invalid://endpoint:  12efg"

	clientConfigMultiValid := confighttp.NewDefaultClientConfig()
	clientConfigMultiValid.Endpoint = "https://localhost:80"

	clientConfigMultiInvalid := confighttp.NewDefaultClientConfig()
	clientConfigMultiInvalid.Endpoint = "invalid://endpoint:  12efg"

	clientConfigMissingScheme := confighttp.NewDefaultClientConfig()
	clientConfigMissingScheme.Endpoint = "www.opentelemetry.io/docs"

	clientConfigValid1 := confighttp.NewDefaultClientConfig()
	clientConfigValid1.Endpoint = "https://opentelemetry.io"

	clientConfigValid2 := confighttp.NewDefaultClientConfig()
	clientConfigValid2.Endpoint = "https://opentelemetry.io:80/docs"

	clientConfigMissingBoth := confighttp.NewDefaultClientConfig()

	clientConfigInvalidSingle := confighttp.NewDefaultClientConfig()
	clientConfigInvalidSingle.Endpoint = "invalid://endpoint:  12efg"

	clientConfigMissingSchemeSingle := confighttp.NewDefaultClientConfig()
	clientConfigMissingSchemeSingle.Endpoint = "www.opentelemetry.io/docs"

	clientConfigValidSingle := confighttp.NewDefaultClientConfig()
	clientConfigValidSingle.Endpoint = "https://opentelemetry.io"

	clientConfigAutoContentTypeEnabled := confighttp.NewDefaultClientConfig()
	clientConfigAutoContentTypeEnabled.Endpoint = "https://opentelemetry.io"

	clientConfigAutoContentTypeDisabled := confighttp.NewDefaultClientConfig()
	clientConfigAutoContentTypeDisabled.Endpoint = "https://opentelemetry.io"

	clientConfigAutoContentTypeDefault := confighttp.NewDefaultClientConfig()
	clientConfigAutoContentTypeDefault.Endpoint = "https://opentelemetry.io"

	testCases := []struct {
		desc        string
		cfg         *Config
		expectedErr error
	}{
		{
			desc: "missing endpoint",
			cfg: &Config{
				Targets: []*targetConfig{
					{
						ClientConfig: clientConfigMissingEndpoint,
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: multierr.Combine(
				errMissingEndpoint,
			),
		},
		{
			desc: "invalid endpoint",
			cfg: &Config{
				Targets: []*targetConfig{
					{
						ClientConfig: clientConfigInvalidEndpoint,
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: multierr.Combine(
				fmt.Errorf("%w: %s", errInvalidEndpoint, `parse "invalid://endpoint:  12efg": invalid port ":  12efg" after host`),
			),
		},
		{
			desc: "invalid config with multiple targets",
			cfg: &Config{
				Targets: []*targetConfig{
					{
						ClientConfig: clientConfigMultiValid,
					},
					{
						ClientConfig: clientConfigMultiInvalid,
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: multierr.Combine(
				fmt.Errorf("%w: %s", errInvalidEndpoint, `parse "invalid://endpoint:  12efg": invalid port ":  12efg" after host`),
			),
		},
		{
			desc: "missing scheme",
			cfg: &Config{
				Targets: []*targetConfig{
					{
						ClientConfig: clientConfigMissingScheme,
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: multierr.Combine(
				fmt.Errorf("%w: %s", errInvalidEndpoint, `parse "www.opentelemetry.io/docs": invalid URI for request`),
			),
		},
		{
			desc: "valid config",
			cfg: &Config{
				Targets: []*targetConfig{
					{
						ClientConfig: clientConfigValid1,
					},
					{
						ClientConfig: clientConfigValid2,
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: nil,
		},
		{
			desc: "missing both endpoint and endpoints",
			cfg: &Config{
				Targets: []*targetConfig{
					{
						ClientConfig: clientConfigMissingBoth,
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: multierr.Combine(
				errMissingEndpoint,
			),
		},
		{
			desc: "invalid single endpoint",
			cfg: &Config{
				Targets: []*targetConfig{
					{
						ClientConfig: clientConfigInvalidSingle,
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: multierr.Combine(
				fmt.Errorf("%w: %s", errInvalidEndpoint, `parse "invalid://endpoint:  12efg": invalid port ":  12efg" after host`),
			),
		},
		{
			desc: "invalid endpoint in endpoints list",
			cfg: &Config{
				Targets: []*targetConfig{
					{
						Endpoints: []string{
							"https://valid.endpoint",
							"invalid://endpoint:  12efg",
						},
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: multierr.Combine(
				fmt.Errorf("%w: %s", errInvalidEndpoint, `parse "invalid://endpoint:  12efg": invalid port ":  12efg" after host`),
			),
		},
		{
			desc: "missing scheme in single endpoint",
			cfg: &Config{
				Targets: []*targetConfig{
					{
						ClientConfig: clientConfigMissingSchemeSingle,
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: multierr.Combine(
				fmt.Errorf("%w: %s", errInvalidEndpoint, `parse "www.opentelemetry.io/docs": invalid URI for request`),
			),
		},
		{
			desc: "valid single endpoint",
			cfg: &Config{
				Targets: []*targetConfig{
					{
						ClientConfig: clientConfigValidSingle,
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: nil,
		},
		{
			desc: "valid endpoints list",
			cfg: &Config{
				Targets: []*targetConfig{
					{
						Endpoints: []string{
							"https://opentelemetry.io",
							"https://opentelemetry.io:80/docs",
						},
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: nil,
		},
		{
			desc: "valid config with auto_content_type enabled",
			cfg: &Config{
				Targets: []*targetConfig{
					{
						ClientConfig:    clientConfigAutoContentTypeEnabled,
						Body:            `{"key": "value"}`,
						AutoContentType: true,
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: nil,
		},
		{
			desc: "valid config with auto_content_type disabled",
			cfg: &Config{
				Targets: []*targetConfig{
					{
						ClientConfig:    clientConfigAutoContentTypeDisabled,
						Body:            `{"key": "value"}`,
						AutoContentType: false,
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: nil,
		},
		{
			desc: "valid config with auto_content_type default (zero value)",
			cfg: &Config{
				Targets: []*targetConfig{
					{
						ClientConfig: clientConfigAutoContentTypeDefault,
						Body:         `{"key": "value"}`,
						// AutoContentType not set (zero value = false)
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			actualErr := tc.cfg.Validate()
			if tc.expectedErr != nil {
				require.EqualError(t, actualErr, tc.expectedErr.Error())
			} else {
				require.NoError(t, actualErr)
			}
		})
	}
}
