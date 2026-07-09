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
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	clientConfigMissingEndpoint.MaxIdleConns = 0
	clientConfigMissingEndpoint.IdleConnTimeout = 0
	clientConfigMissingEndpoint.ForceAttemptHTTP2 = false

	clientConfigInvalidEndpoint := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	clientConfigInvalidEndpoint.MaxIdleConns = 0
	clientConfigInvalidEndpoint.IdleConnTimeout = 0
	clientConfigInvalidEndpoint.ForceAttemptHTTP2 = false
	clientConfigInvalidEndpoint.Endpoint = "invalid://endpoint:  12efg"

	clientConfigMultiValid := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	clientConfigMultiValid.MaxIdleConns = 0
	clientConfigMultiValid.IdleConnTimeout = 0
	clientConfigMultiValid.ForceAttemptHTTP2 = false
	clientConfigMultiValid.Endpoint = "https://localhost:80"

	clientConfigMultiInvalid := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	clientConfigMultiInvalid.MaxIdleConns = 0
	clientConfigMultiInvalid.IdleConnTimeout = 0
	clientConfigMultiInvalid.ForceAttemptHTTP2 = false
	clientConfigMultiInvalid.Endpoint = "invalid://endpoint:  12efg"

	clientConfigMissingScheme := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	clientConfigMissingScheme.MaxIdleConns = 0
	clientConfigMissingScheme.IdleConnTimeout = 0
	clientConfigMissingScheme.ForceAttemptHTTP2 = false
	clientConfigMissingScheme.Endpoint = "www.opentelemetry.io/docs"

	clientConfigValid1 := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	clientConfigValid1.MaxIdleConns = 0
	clientConfigValid1.IdleConnTimeout = 0
	clientConfigValid1.ForceAttemptHTTP2 = false
	clientConfigValid1.Endpoint = "https://opentelemetry.io"

	clientConfigValid2 := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	clientConfigValid2.MaxIdleConns = 0
	clientConfigValid2.IdleConnTimeout = 0
	clientConfigValid2.ForceAttemptHTTP2 = false
	clientConfigValid2.Endpoint = "https://opentelemetry.io:80/docs"

	clientConfigMissingBoth := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	clientConfigMissingBoth.MaxIdleConns = 0
	clientConfigMissingBoth.IdleConnTimeout = 0
	clientConfigMissingBoth.ForceAttemptHTTP2 = false

	clientConfigInvalidSingle := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	clientConfigInvalidSingle.MaxIdleConns = 0
	clientConfigInvalidSingle.IdleConnTimeout = 0
	clientConfigInvalidSingle.ForceAttemptHTTP2 = false
	clientConfigInvalidSingle.Endpoint = "invalid://endpoint:  12efg"

	clientConfigMissingSchemeSingle := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	clientConfigMissingSchemeSingle.MaxIdleConns = 0
	clientConfigMissingSchemeSingle.IdleConnTimeout = 0
	clientConfigMissingSchemeSingle.ForceAttemptHTTP2 = false
	clientConfigMissingSchemeSingle.Endpoint = "www.opentelemetry.io/docs"

	clientConfigValidSingle := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	clientConfigValidSingle.MaxIdleConns = 0
	clientConfigValidSingle.IdleConnTimeout = 0
	clientConfigValidSingle.ForceAttemptHTTP2 = false
	clientConfigValidSingle.Endpoint = "https://opentelemetry.io"

	clientConfigAutoContentTypeEnabled := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	clientConfigAutoContentTypeEnabled.MaxIdleConns = 0
	clientConfigAutoContentTypeEnabled.IdleConnTimeout = 0
	clientConfigAutoContentTypeEnabled.ForceAttemptHTTP2 = false
	clientConfigAutoContentTypeEnabled.Endpoint = "https://opentelemetry.io"

	clientConfigAutoContentTypeDisabled := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	clientConfigAutoContentTypeDisabled.MaxIdleConns = 0
	clientConfigAutoContentTypeDisabled.IdleConnTimeout = 0
	clientConfigAutoContentTypeDisabled.ForceAttemptHTTP2 = false
	clientConfigAutoContentTypeDisabled.Endpoint = "https://opentelemetry.io"

	clientConfigAutoContentTypeDefault := confighttp.NewDefaultClientConfig()
	// TODO: See https://github.com/open-telemetry/opentelemetry-collector-contrib/issues/49316.
	clientConfigAutoContentTypeDefault.MaxIdleConns = 0
	clientConfigAutoContentTypeDefault.IdleConnTimeout = 0
	clientConfigAutoContentTypeDefault.ForceAttemptHTTP2 = false
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
