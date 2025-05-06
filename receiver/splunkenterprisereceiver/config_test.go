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

var dummyID = component.MustNewID("dummy")

func TestEndpointCorrectness(t *testing.T) {
	// Declare errors for tests that should fail
	var errBad, errScheme error
	// Error for bad or missing endpoint
	errBad = multierr.Append(errBad, errBadOrMissingEndpoint)
	// There is no way with the current SDK design to create a test config that
	// satisfies the auth extension so we will just expect this error to appear.
	errBad = multierr.Append(errBad, errMissingAuthExtension)

	// Error related to bad scheme (not http/s)
	errScheme = multierr.Append(errScheme, errBadScheme)
	errScheme = multierr.Append(errScheme, errMissingAuthExtension)

	tests := []struct {
		desc     string
		expected error
		config   *Config
	}{
		{
			desc:     "missing any endpoint setting",
			expected: errBad,
			config: &Config{
				IdxEndpoint: confighttp.ClientConfig{
					Auth: &configauth.Config{AuthenticatorID: dummyID},
				},
				SHEndpoint: confighttp.ClientConfig{
					Auth: &configauth.Config{AuthenticatorID: dummyID},
				},
				CMEndpoint: confighttp.ClientConfig{
					Auth: &configauth.Config{AuthenticatorID: dummyID},
				},
			},
		},
		{
			desc:     "properly configured invalid endpoint",
			expected: errBad,
			config: &Config{
				IdxEndpoint: confighttp.ClientConfig{
					Auth:     &configauth.Config{AuthenticatorID: dummyID},
					Endpoint: "123.321.12.1:1",
				},
			},
		},
		{
			desc:     "properly configured endpoint has bad scheme",
			expected: errScheme,
			config: &Config{
				IdxEndpoint: confighttp.ClientConfig{
					Auth:     &configauth.Config{AuthenticatorID: dummyID},
					Endpoint: "gss://123.124.32.12:90",
				},
			},
		},
		{
			desc:     "properly configured endpoint missing auth",
			expected: errMissingAuthExtension,
			config: &Config{
				IdxEndpoint: confighttp.ClientConfig{
					Endpoint: "https://123.123.32.2:2093",
				},
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
