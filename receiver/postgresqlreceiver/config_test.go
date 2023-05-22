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
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/postgresqlreceiver/internal/metadata"
)

func TestValidate(t *testing.T) {
	testCases := []struct {
		desc                  string
		defaultConfigModifier func(cfg *Config)
		expected              error
	}{
		{
			desc:                  "missing username and password",
			defaultConfigModifier: func(cfg *Config) {},
			expected: multierr.Combine(
				errors.New(ErrNoUsername),
				errors.New(ErrNoPassword),
			),
		},
		{
			desc: "missing password",
			defaultConfigModifier: func(cfg *Config) {
				cfg.Username = "otel"
			},
			expected: multierr.Combine(
				errors.New(ErrNoPassword),
			),
		},
		{
			desc: "missing username",
			defaultConfigModifier: func(cfg *Config) {
				cfg.Password = "otel"
			},
			expected: multierr.Combine(
				errors.New(ErrNoUsername),
			),
		},
		{
			desc: "bad endpoint",
			defaultConfigModifier: func(cfg *Config) {
				cfg.Username = "otel"
				cfg.Password = "otel"
				cfg.Endpoint = "open-telemetry"
			},
			expected: multierr.Combine(
				errors.New(ErrHostPort),
			),
		},
		{
			desc: "bad transport",
			defaultConfigModifier: func(cfg *Config) {
				cfg.Username = "otel"
				cfg.Password = "otel"
				cfg.Transport = "teacup"
			},
			expected: multierr.Combine(
				errors.New(ErrTransportsSupported),
			),
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
			expected: multierr.Combine(
				fmt.Errorf(ErrNotSupported, "ServerName"),
				fmt.Errorf(ErrNotSupported, "MaxVersion"),
				fmt.Errorf(ErrNotSupported, "MinVersion"),
			),
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
			actual := component.ValidateConfig(cfg)
			require.Equal(t, tC.expected, actual)
		})
	}
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	t.Run("postgresql", func(t *testing.T) {
		sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
		require.NoError(t, err)
		require.NoError(t, component.UnmarshalConfig(sub, cfg))

		expected := factory.CreateDefaultConfig().(*Config)
		expected.Endpoint = "localhost:5432"
		expected.Username = "otel"
		expected.Password = "${env:POSTGRESQL_PASSWORD}"

		require.Equal(t, expected, cfg)
	})

	t.Run("postgresql/all", func(t *testing.T) {
		sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "all").String())
		require.NoError(t, err)
		require.NoError(t, component.UnmarshalConfig(sub, cfg))

		expected := factory.CreateDefaultConfig().(*Config)
		expected.Endpoint = "localhost:5432"
		expected.NetAddr.Transport = "tcp"
		expected.Username = "otel"
		expected.Password = "${env:POSTGRESQL_PASSWORD}"
		expected.Databases = []string{"otel"}
		expected.CollectionInterval = 10 * time.Second
		expected.TLSClientSetting = configtls.TLSClientSetting{
			Insecure:           false,
			InsecureSkipVerify: false,
			TLSSetting: configtls.TLSSetting{
				CAFile:   "/home/otel/authorities.crt",
				CertFile: "/home/otel/mypostgrescert.crt",
				KeyFile:  "/home/otel/mypostgreskey.key",
			},
		}

		require.Equal(t, expected, cfg)
	})
}
