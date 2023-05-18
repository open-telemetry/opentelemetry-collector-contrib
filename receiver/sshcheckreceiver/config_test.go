// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sshcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sshcheckreceiver"

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/sshcheckreceiver/internal/configssh"
)

// check that OTel Collector patterns are implemented
func TestCheckConfig(t *testing.T) {
	t.Parallel()
	if err := componenttest.CheckConfigStruct(&Config{}); err != nil {
		t.Fatal(err)
	}
}

// test the validate function for config
func TestValidate(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		desc        string
		cfg         *Config
		expectedErr error
	}{
		{
			desc: "missing password and keyfile",
			cfg: &Config{
				SSHClientSettings: configssh.SSHClientSettings{
					Username: "otelu",
					Endpoint: "goodhost:2222",
				},
			},
			expectedErr: multierr.Combine(errMissingPasswordAndKeyFile),
		},
		{
			desc: "missing username",
			cfg: &Config{
				SSHClientSettings: configssh.SSHClientSettings{
					Endpoint: "goodhost:2222",
					KeyFile:  "/home/.ssh/id_rsa",
				},
			},
			expectedErr: multierr.Combine(
				errMissingUsername,
			),
		},
		{
			desc: "invalid endpoint",
			cfg: &Config{
				SSHClientSettings: configssh.SSHClientSettings{
					Endpoint: "badendpoint . cuz spaces:2222",
					Username: "otelu",
					Password: "otelp",
				},
			},
			expectedErr: multierr.Combine(
				errInvalidEndpoint,
			),
		},
		{
			desc: "no error with password",
			cfg: &Config{
				SSHClientSettings: configssh.SSHClientSettings{
					Endpoint: "localhost:2222",
					Username: "otelu",
					Password: "otelp",
				},
			},
			expectedErr: error(nil),
		},
		{
			desc: "no error with keyfile",
			cfg: &Config{
				SSHClientSettings: configssh.SSHClientSettings{
					Endpoint: "localhost:2222",
					Username: "otelu",
					KeyFile:  "/possibly/a_path",
				},
			},
			expectedErr: error(nil),
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
