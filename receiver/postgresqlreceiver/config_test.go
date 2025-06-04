// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package postgresqlreceiver

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver/internal/metadata"
)

func TestValidate(t *testing.T) {
	testCases := []struct {
		desc                  string
		defaultConfigModifier func(cfg *Config)
		expected              []error
	}{
		{
			desc:                  "missing username and password",
			defaultConfigModifier: func(*Config) {},
			expected: []error{
				errors.New(ErrNoUsername),
				errors.New(ErrNoPassword),
			},
		},
		{
			desc: "missing password",
			defaultConfigModifier: func(cfg *Config) {
				cfg.Username = "otel"
			},
			expected: []error{
				errors.New(ErrNoPassword),
			},
		},
		{
			desc: "missing username",
			defaultConfigModifier: func(cfg *Config) {
				cfg.Password = "otel"
			},
			expected: []error{
				errors.New(ErrNoUsername),
			},
		},
		{
			desc: "bad endpoint",
			defaultConfigModifier: func(cfg *Config) {
				cfg.Username = "otel"
				cfg.Password = "otel"
				cfg.Endpoint = "open-telemetry"
			},
			expected: []error{
				errors.New(ErrHostPort),
			},
		},
		{
			desc: "bad transport",
			defaultConfigModifier: func(cfg *Config) {
				cfg.Username = "otel"
				cfg.Password = "otel"
				cfg.Transport = "udp"
			},
			expected: []error{
				errors.New(ErrTransportsSupported),
			},
		},
		{
			desc: "unsupported SSL params",
			defaultConfigModifier: func(cfg *Config) {
				cfg.Username = "otel"
				cfg.Password = "otel"
				cfg.ServerName = "notlocalhost"
				cfg.MinVersion = "1.0"
				cfg.MaxVersion = "1.0"
			},
			expected: []error{
				fmt.Errorf(ErrNotSupported, "ServerName"),
				fmt.Errorf(ErrNotSupported, "MaxVersion"),
				fmt.Errorf(ErrNotSupported, "MinVersion"),
			},
		},
		{
			desc: "no error",
			defaultConfigModifier: func(cfg *Config) {
				cfg.Username = "otel"
				cfg.Password = "otel"
			},
			expected: nil,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			factory := NewFactory()
			cfg := factory.CreateDefaultConfig().(*Config)
			tC.defaultConfigModifier(cfg)
			actual := xconfmap.Validate(cfg)
			if len(tC.expected) > 0 {
				for _, err := range tC.expected {
					require.ErrorContains(t, actual, err.Error())
				}
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	cm, confErr := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, confErr)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	t.Run("postgresql/minimal", func(t *testing.T) {
		sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "minimal").String())
		require.NoError(t, err)
		require.NoError(t, sub.Unmarshal(cfg))

		expected := factory.CreateDefaultConfig().(*Config)
		expected.Endpoint = "localhost:5432"
		expected.Username = "otel"
		expected.Password = "${env:POSTGRESQL_PASSWORD}"
		expected.QuerySampleCollection.Enabled = true
		expected.TopNQuery = 1234
		expected.TopQueryCollection.Enabled = true
		require.Equal(t, expected, cfg)
	})

	cfg = factory.CreateDefaultConfig()

	t.Run("postgresql/pool", func(t *testing.T) {
		sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "pool").String())
		require.NoError(t, err)
		require.NoError(t, sub.Unmarshal(cfg))

		expected := factory.CreateDefaultConfig().(*Config)
		expected.Endpoint = "localhost:5432"
		expected.Transport = confignet.TransportTypeTCP
		expected.Username = "otel"
		expected.Password = "${env:POSTGRESQL_PASSWORD}"
		expected.ConnectionPool = ConnectionPool{
			MaxIdleTime: ptr(30 * time.Second),
			MaxIdle:     ptr(5),
		}

		require.Equal(t, expected, cfg)
	})

	cfg = factory.CreateDefaultConfig()

	t.Run("postgresql/all", func(t *testing.T) {
		sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "all").String())
		require.NoError(t, err)
		require.NoError(t, sub.Unmarshal(cfg))

		expected := factory.CreateDefaultConfig().(*Config)
		expected.Endpoint = "localhost:5432"
		expected.Transport = confignet.TransportTypeTCP
		expected.Username = "otel"
		expected.Password = "${env:POSTGRESQL_PASSWORD}"
		expected.Databases = []string{"otel"}
		expected.ExcludeDatabases = []string{"template0"}
		expected.CollectionInterval = 10 * time.Second
		expected.ClientConfig = configtls.ClientConfig{
			Insecure:           false,
			InsecureSkipVerify: false,
			Config: configtls.Config{
				CAFile:   "/home/otel/authorities.crt",
				CertFile: "/home/otel/mypostgrescert.crt",
				KeyFile:  "/home/otel/mypostgreskey.key",
			},
		}
		expected.ConnectionPool = ConnectionPool{
			MaxIdleTime: ptr(30 * time.Second),
			MaxLifetime: ptr(time.Minute),
			MaxIdle:     ptr(5),
			MaxOpen:     ptr(10),
		}

		require.Equal(t, expected, cfg)
	})
}

func ptr[T any](value T) *T {
	return &value
}
