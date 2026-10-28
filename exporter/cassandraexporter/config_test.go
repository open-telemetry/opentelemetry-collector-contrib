// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cassandraexporter

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/confmap"
	"go.opentelemetry.io/collector/confmap/confmaptest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/cassandraexporter/internal/metadata"
)

const (
	defaultDSN  = "127.0.0.1"
	defaultPort = 9042
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	defaultCfg := createDefaultConfig()
	defaultCfg.(*Config).DSN = defaultDSN
	defaultCfg.(*Config).Port = defaultPort

	tests := []struct {
		id          component.ID
		expected    component.Config
		expectedErr string
	}{
		{
			id:       component.NewIDWithName(metadata.Type, ""),
			expected: defaultCfg,
		},
		{
			id: component.NewIDWithName(metadata.Type, "custom"),
			expected: &Config{
				DSN:        "10.0.0.1",
				Port:       9043,
				Timeout:    5 * time.Second,
				Keyspace:   "custom_keyspace",
				TraceTable: "custom_spans",
				LogsTable:  "custom_logs",
				Replication: Replication{
					Class:             "NetworkTopologyStrategy",
					ReplicationFactor: 3,
				},
				Compression: Compression{
					Algorithm: "ZstdCompressor",
				},
				Auth: Auth{
					UserName: "user",
					Password: "password",
				},
			},
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_dsn"),
			expectedErr: "dsn must not be empty",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_port_low"),
			expectedErr: "port must be between 1 and 65535",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_port_high"),
			expectedErr: "port must be between 1 and 65535",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_timeout"),
			expectedErr: "timeout must be positive",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_keyspace"),
			expectedErr: "keyspace must not be empty",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_trace_table"),
			expectedErr: "trace_table must not be empty",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_logs_table"),
			expectedErr: "logs_table must not be empty",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_replication_class"),
			expectedErr: "invalid replication: class must not be empty",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_replication_factor"),
			expectedErr: "invalid replication: replication_factor must be positive",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_compression_algorithm"),
			expectedErr: "invalid compression: algorithm must not be empty",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_auth_missing_username"),
			expectedErr: "empty auth.username",
		},
		{
			id:          component.NewIDWithName(metadata.Type, "invalid_auth_missing_password"),
			expectedErr: "empty auth.password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.id.String(), func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig()

			sub, err := cm.Sub(tt.id.String())
			require.NoError(t, err)
			require.NoError(t, sub.Unmarshal(cfg))

			err = confmap.Validate(cfg)
			if tt.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expected, cfg)
			}
		})
	}
}

func TestConfigValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		mutate      func(*Config)
		expectedErr string
	}{
		{
			name:   "valid default config",
			mutate: nil,
		},
		{
			name: "valid custom config with auth",
			mutate: func(c *Config) {
				c.Auth = Auth{
					UserName: "user",
					Password: "password",
				}
			},
		},
		{
			name: "empty dsn",
			mutate: func(c *Config) {
				c.DSN = ""
			},
			expectedErr: "dsn must not be empty",
		},
		{
			name: "zero port",
			mutate: func(c *Config) {
				c.Port = 0
			},
			expectedErr: "port must be between 1 and 65535",
		},
		{
			name: "negative port",
			mutate: func(c *Config) {
				c.Port = -1
			},
			expectedErr: "port must be between 1 and 65535",
		},
		{
			name: "port above maximum",
			mutate: func(c *Config) {
				c.Port = 65536
			},
			expectedErr: "port must be between 1 and 65535",
		},
		{
			name: "zero timeout",
			mutate: func(c *Config) {
				c.Timeout = 0
			},
			expectedErr: "timeout must be positive",
		},
		{
			name: "negative timeout",
			mutate: func(c *Config) {
				c.Timeout = -5 * time.Second
			},
			expectedErr: "timeout must be positive",
		},
		{
			name: "empty keyspace",
			mutate: func(c *Config) {
				c.Keyspace = ""
			},
			expectedErr: "keyspace must not be empty",
		},
		{
			name: "empty trace table",
			mutate: func(c *Config) {
				c.TraceTable = ""
			},
			expectedErr: "trace_table must not be empty",
		},
		{
			name: "empty logs table",
			mutate: func(c *Config) {
				c.LogsTable = ""
			},
			expectedErr: "logs_table must not be empty",
		},
		{
			name: "empty replication class",
			mutate: func(c *Config) {
				c.Replication.Class = ""
			},
			expectedErr: "invalid replication: class must not be empty",
		},
		{
			name: "zero replication factor",
			mutate: func(c *Config) {
				c.Replication.ReplicationFactor = 0
			},
			expectedErr: "invalid replication: replication_factor must be positive",
		},
		{
			name: "negative replication factor",
			mutate: func(c *Config) {
				c.Replication.ReplicationFactor = -2
			},
			expectedErr: "invalid replication: replication_factor must be positive",
		},
		{
			name: "empty compression algorithm",
			mutate: func(c *Config) {
				c.Compression.Algorithm = ""
			},
			expectedErr: "invalid compression: algorithm must not be empty",
		},
		{
			name: "auth missing password",
			mutate: func(c *Config) {
				c.Auth = Auth{
					UserName: "user",
					Password: "",
				}
			},
			expectedErr: "empty auth.password",
		},
		{
			name: "auth missing username",
			mutate: func(c *Config) {
				c.Auth = Auth{
					UserName: "",
					Password: "password",
				}
			},
			expectedErr: "empty auth.username",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			if tt.mutate != nil {
				tt.mutate(cfg)
			}
			err := cfg.Validate()
			if tt.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestReplicationValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		replication Replication
		expectedErr string
	}{
		{
			name: "valid",
			replication: Replication{
				Class:             "SimpleStrategy",
				ReplicationFactor: 1,
			},
		},
		{
			name: "empty class",
			replication: Replication{
				Class:             "",
				ReplicationFactor: 1,
			},
			expectedErr: "class must not be empty",
		},
		{
			name: "zero replication factor",
			replication: Replication{
				Class:             "SimpleStrategy",
				ReplicationFactor: 0,
			},
			expectedErr: "replication_factor must be positive",
		},
		{
			name: "negative replication factor",
			replication: Replication{
				Class:             "SimpleStrategy",
				ReplicationFactor: -1,
			},
			expectedErr: "replication_factor must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.replication.Validate()
			if tt.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCompressionValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		compression Compression
		expectedErr string
	}{
		{
			name: "valid",
			compression: Compression{
				Algorithm: "LZ4Compressor",
			},
		},
		{
			name: "empty algorithm",
			compression: Compression{
				Algorithm: "",
			},
			expectedErr: "algorithm must not be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.compression.Validate()
			if tt.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAuthValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		auth        Auth
		expectedErr string
	}{
		{
			name: "valid empty",
			auth: Auth{},
		},
		{
			name: "valid populated",
			auth: Auth{
				UserName: "user",
				Password: "password",
			},
		},
		{
			name: "missing password",
			auth: Auth{
				UserName: "user",
			},
			expectedErr: "empty auth.password",
		},
		{
			name: "missing username",
			auth: Auth{
				Password: "password",
			},
			expectedErr: "empty auth.username",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.auth.Validate()
			if tt.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
