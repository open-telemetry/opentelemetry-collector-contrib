// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package splunkenterprisereceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver"

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configauth"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/splunkenterprisereceiver/internal/metadata"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)
	id := component.NewID(metadata.Type)
	_, err = cm.Sub(id.String())
	require.NoError(t, err)
}

func TestEndpointCorrectness(t *testing.T) {
	// Declare errors for tests that should fail
	var errBad, errMisconf, errScheme error
	// Error for bad or missing endpoint
	errBad = multierr.Append(errBad, errBadOrMissingEndpoint)
	// There is no way with the current SDK design to create a test config that
	// satisfies the auth extention so we will just expect this error to appear.
	errBad = multierr.Append(errBad, errMissingAuthExtension)

	// Errors related to setting the wrong endpoint field (i.e. the one from httpconfig)
	errMisconf = multierr.Append(errMisconf, errUnspecifiedEndpoint)
	errMisconf = multierr.Append(errMisconf, errMissingAuthExtension)

	// Error related to bad scheme (not http/s)
	errScheme = multierr.Append(errScheme, errBadScheme)
	errScheme = multierr.Append(errScheme, errMissingAuthExtension)

	httpCfg := confighttp.NewDefaultHTTPClientSettings()
	httpCfg.Auth = &configauth.Authentication{AuthenticatorID: component.NewID("dummy")}
	httpCfgWithEndpoint := httpCfg
	httpCfgWithEndpoint.Endpoint = "https://123.123.32.2:2093"
	httpCfgWithEndpoint.Auth = &configauth.Authentication{AuthenticatorID: component.NewID("dummy")}

	tests := []struct {
		desc     string
		expected error
		config   *Config
	}{
		{
			desc:     "missing any endpoint setting",
			expected: errBad,
			config: &Config{
				HTTPClientSettings: httpCfg,
			},
		},
		{
			desc:     "configured the wrong endpoint field (httpconfig.Endpoint)",
			expected: errMisconf,
			config: &Config{
				HTTPClientSettings: httpCfgWithEndpoint,
			},
		},
		{
			desc:     "properly configured invalid endpoint",
			expected: errBad,
			config: &Config{
				HTTPClientSettings: httpCfg,
				IdxEndpoint:        "123.12.23.43:80",
			},
		},
		{
			desc:     "properly configured endpoint has bad scheme",
			expected: errScheme,
			config: &Config{
				HTTPClientSettings: httpCfg,
				IdxEndpoint:        "gss://123.124.32.12:90",
			},
		},
		{
			desc:     "properly configured endpoint",
			expected: errMissingAuthExtension,
			config: &Config{
				HTTPClientSettings: httpCfg,
				IdxEndpoint:        "https://123.123.32.2:2093",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := test.config.Validate()
			t.Logf("%v\n", err)
			require.Error(t, err)
			require.Contains(t, test.expected.Error(), err.Error())
		})
	}
}
