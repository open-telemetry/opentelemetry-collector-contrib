// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cassandraexporter

import (
	"context"
	"errors"
	"testing"

	gocql "github.com/apache/cassandra-gocql-driver/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
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

// TestPushLogsData_EmptyLogs verifies behavior with valid but empty logs
func TestPushLogsData_EmptyLogs(t *testing.T) {
	exporter := &logsExporter{
		logger: zap.NewNop(),
		cfg:    createDefaultConfig().(*Config),
		client: nil, // no client needed for empty logs
	}

	// empty logs should succeed without trying to insert
	logs := plog.NewLogs()
	err := exporter.pushLogsData(context.Background(), logs)

	assert.NoError(t, err, "empty logs should not cause errors")
}
