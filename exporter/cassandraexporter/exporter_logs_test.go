// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cassandraexporter

import (
	"errors"
	"testing"

	"github.com/gocql/gocql"
	"github.com/stretchr/testify/require"
)

func TestNewCluster(t *testing.T) {
	testCases := map[string]struct {
		cfg                   *Config
		expectedAuthenticator gocql.Authenticator
		expectedErr           error
	}{
		"empty_auth": {
			cfg:                   withDefaultConfig(),
			expectedAuthenticator: nil,
		},
		"empty_username": {
			cfg: withDefaultConfig(func(config *Config) {
				config.Auth.Password = "pass"
			}),
			expectedAuthenticator: nil,
			expectedErr:           errors.New("empty auth.username"),
		},
		"empty_password": {
			cfg: withDefaultConfig(func(config *Config) {
				config.Auth.UserName = "user"
			}),
			expectedAuthenticator: nil,
			expectedErr:           errors.New("empty auth.password"),
		},
		"success_auth": {
			cfg: withDefaultConfig(func(config *Config) {
				config.Auth.UserName = "user"
				config.Auth.Password = "pass"
			}),
			expectedAuthenticator: gocql.PasswordAuthenticator{
				Username: "user",
				Password: "pass",
			},
		},
	}
	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			c, err := newCluster(test.cfg)
			if err == nil {
				require.Equal(t, test.expectedAuthenticator, c.Authenticator)
			}
			require.Equal(t, test.expectedErr, err)
		})
	}
}

func withDefaultConfig(fns ...func(*Config)) *Config {
	cfg := createDefaultConfig().(*Config)
	for _, fn := range fns {
		fn(cfg)
	}
	return cfg
}
