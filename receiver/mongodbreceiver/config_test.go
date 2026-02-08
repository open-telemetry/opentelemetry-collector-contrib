// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mongodbreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver"

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap/confmaptest"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/scraper/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/mongodbreceiver/internal/metadata"
)

func TestValidate(t *testing.T) {
	testCases := []struct {
		endpoints []string
		desc      string
		username  string
		password  string
		expected  error
	}{
		{
			desc:      "no username, no password",
			endpoints: []string{"localhost:27107"},
			username:  "",
			password:  "",
			expected:  nil,
		},
		{
			desc:      "no username, with password",
			endpoints: []string{"localhost:27107"},
			username:  "",
			password:  "pass",
			expected:  errors.New("password provided without user"),
		},
		{
			desc:      "with username, no password",
			endpoints: []string{"localhost:27107"},
			username:  "user",
			password:  "",
			expected:  errors.New("username provided without password"),
		},
		{
			desc:      "with username and password",
			endpoints: []string{"localhost:27107"},
			username:  "user",
			password:  "pass",
			expected:  nil,
		},
		{
			desc:     "no hosts",
			username: "user",
			password: "pass",
			expected: errors.New("no hosts were specified in the config"),
		},
		{
			desc:      "valid hostname",
			endpoints: []string{"localhost"},
			expected:  nil,
		},
		{
			desc:      "empty host",
			username:  "user",
			endpoints: []string{""},
			expected:  errors.New("no endpoint specified for one of the hosts"),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			var hosts []confignet.TCPAddrConfig

			for _, ep := range tc.endpoints {
				hosts = append(hosts, confignet.TCPAddrConfig{
					Endpoint: ep,
				})
			}

			cfg := &Config{
				Username:         tc.username,
				Password:         configopaque.String(tc.password),
				Hosts:            hosts,
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			}
			err := xconfmap.Validate(cfg)
			if tc.expected == nil {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.expected.Error())
			}
		})
	}
}

func TestBadTLSConfigs(t *testing.T) {
	testCases := []struct {
		desc        string
		tlsConfig   configtls.ClientConfig
		expectError bool
	}{
		{
			desc: "CA file not found",
			tlsConfig: configtls.ClientConfig{
				Config: configtls.Config{
					CAFile: "not/a/real/file.pem",
				},
				Insecure:           false,
				InsecureSkipVerify: false,
				ServerName:         "",
			},
			expectError: true,
		},
		{
			desc: "no issues",
			tlsConfig: configtls.ClientConfig{
				Config:             configtls.Config{},
				Insecure:           false,
				InsecureSkipVerify: false,
				ServerName:         "",
			},
			expectError: false,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			cfg := &Config{
				Username: "otel",
				Password: "pword",
				Hosts: []confignet.TCPAddrConfig{
					{
						Endpoint: defaultEndpoint,
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
				ClientConfig:     tc.tlsConfig,
			}
			err := xconfmap.Validate(cfg)
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestOptions(t *testing.T) {
	cfg := &Config{
		Hosts: []confignet.TCPAddrConfig{
			{
				Endpoint: defaultEndpoint,
			},
		},
		Username:   "uname",
		Password:   "password",
		Timeout:    2 * time.Minute,
		ReplicaSet: "rs-1",
	}

	clientOptions := cfg.ClientOptions(false)
	require.Equal(t, clientOptions.Auth.Username, cfg.Username)
	require.Equal(t,
		clientOptions.ConnectTimeout.Milliseconds(),
		(2 * time.Minute).Milliseconds(),
	)
	require.Equal(t, "rs-1", *clientOptions.ReplicaSet)
}

func TestOptionsWithAuthMechanismAndSource(t *testing.T) {
	cfg := &Config{
		Hosts: []confignet.TCPAddrConfig{
			{
				Endpoint: defaultEndpoint,
			},
		},
		Username:      "uname",
		Password:      "password",
		AuthMechanism: "SCRAM-SHA-256",
		AuthSource:    "admin",
		AuthMechanismProperties: map[string]string{
			"SERVICE_NAME": "mongodb",
		},
		Timeout:    2 * time.Minute,
		ReplicaSet: "rs-1",
	}

	// Test primary connection options
	clientOptions := cfg.ClientOptions(false)
	require.Equal(t, clientOptions.Auth.Username, cfg.Username)
	require.Equal(t, clientOptions.Auth.AuthMechanism, cfg.AuthMechanism)
	require.Equal(t, clientOptions.Auth.AuthSource, cfg.AuthSource)
	require.Equal(t, clientOptions.Auth.AuthMechanismProperties, cfg.AuthMechanismProperties)
	require.Equal(t,
		clientOptions.ConnectTimeout.Milliseconds(),
		(2 * time.Minute).Milliseconds(),
	)
	require.Equal(t, "rs-1", *clientOptions.ReplicaSet)

	// Test secondary connection options
	secondaryOptions := cfg.ClientOptions(true)
	require.Equal(t, secondaryOptions.Auth.Username, cfg.Username)
	require.Equal(t, secondaryOptions.Auth.AuthMechanism, cfg.AuthMechanism)
	require.Equal(t, secondaryOptions.Auth.AuthSource, cfg.AuthSource)
	require.Equal(t, secondaryOptions.Auth.AuthMechanismProperties, cfg.AuthMechanismProperties)
}

func TestOptionsWithAuthMechanismOnly(t *testing.T) {
	// Test auth mechanisms that don't require username/password (e.g., MONGODB-X509, MONGODB-AWS with IAM)
	cfg := &Config{
		Hosts: []confignet.TCPAddrConfig{
			{
				Endpoint: defaultEndpoint,
			},
		},
		AuthMechanism: "MONGODB-X509",
		AuthSource:    "$external",
		Timeout:       2 * time.Minute,
	}

	// Test primary connection options
	clientOptions := cfg.ClientOptions(false)
	require.Empty(t, clientOptions.Auth.Username)
	require.Empty(t, clientOptions.Auth.Password)
	require.Equal(t, "MONGODB-X509", clientOptions.Auth.AuthMechanism)
	require.Equal(t, "$external", clientOptions.Auth.AuthSource)

	// Test secondary connection options
	secondaryOptions := cfg.ClientOptions(true)
	require.Empty(t, secondaryOptions.Auth.Username)
	require.Empty(t, secondaryOptions.Auth.Password)
	require.Equal(t, "MONGODB-X509", secondaryOptions.Auth.AuthMechanism)
	require.Equal(t, "$external", secondaryOptions.Auth.AuthSource)
}

func TestOptionsTLS(t *testing.T) {
	// loading valid ca file
	caFile := filepath.Join("testdata", "certs", "ca.crt")

	cfg := &Config{
		Hosts: []confignet.TCPAddrConfig{
			{
				Endpoint: defaultEndpoint,
			},
		},
		ClientConfig: configtls.ClientConfig{
			Insecure: false,
			Config: configtls.Config{
				CAFile: caFile,
			},
		},
	}
	opts := cfg.ClientOptions(false)
	require.NotNil(t, opts.TLSConfig)
}

func TestLoadConfig(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	expected := factory.CreateDefaultConfig().(*Config)
	expected.Hosts = []confignet.TCPAddrConfig{
		{
			Endpoint: defaultEndpoint,
		},
	}
	expected.Username = "otel"
	expected.Password = "${env:MONGO_PASSWORD}"
	expected.CollectionInterval = time.Minute
	expected.AuthMechanism = "SCRAM-SHA-256"
	expected.AuthSource = "admin"
	expected.AuthMechanismProperties = map[string]string{
		"SERVICE_NAME": "mongodb",
	}

	require.Equal(t, expected, cfg)
}
