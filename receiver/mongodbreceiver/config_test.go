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
		scheme    string
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
		{
			desc:      "scheme mongodb is valid",
			endpoints: []string{"localhost:27017"},
			scheme:    "mongodb",
			expected:  nil,
		},
		{
			desc:      "scheme mongodb+srv with one host is valid",
			endpoints: []string{"cluster0.example.mongodb.net"},
			scheme:    "mongodb+srv",
			expected:  nil,
		},
		{
			desc:      "scheme mongodb+srv with multiple hosts is invalid",
			endpoints: []string{"host1.example.net", "host2.example.net"},
			scheme:    "mongodb+srv",
			expected:  errors.New("mongodb+srv scheme requires exactly one host"),
		},
		{
			desc:      "invalid scheme",
			endpoints: []string{"localhost:27017"},
			scheme:    "invalid",
			expected:  errors.New("invalid scheme \"invalid\", must be \"mongodb\" or \"mongodb+srv\""),
		},
		{
			desc:      "empty scheme defaults to mongodb",
			endpoints: []string{"localhost:27017"},
			scheme:    "",
			expected:  nil,
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
				Scheme:           tc.scheme,
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

func TestOptionsDefaultScheme(t *testing.T) {
	cfg := &Config{
		Hosts: []confignet.TCPAddrConfig{
			{
				Endpoint: "localhost:27017",
			},
		},
	}

	clientOptions := cfg.ClientOptions(false)
	require.Equal(t, []string{"localhost:27017"}, clientOptions.Hosts)
}

func TestOptionsSRVScheme(t *testing.T) {
	cfg := &Config{
		Hosts: []confignet.TCPAddrConfig{
			{
				Endpoint: "cluster0.example.mongodb.net",
			},
		},
		Scheme: "mongodb+srv",
	}

	clientOptions := cfg.ClientOptions(false)
	require.NotNil(t, clientOptions)
	// mongodb+srv:// defers host resolution to connect time,
	// so Hosts is not populated at parse time
	require.Nil(t, clientOptions.Hosts)
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

	require.Equal(t, expected, cfg)
}

func TestLoadConfigSRV(t *testing.T) {
	cm, err := confmaptest.LoadConf(filepath.Join("testdata", "config.yaml"))
	require.NoError(t, err)

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()

	sub, err := cm.Sub(component.NewIDWithName(metadata.Type, "srv").String())
	require.NoError(t, err)
	require.NoError(t, sub.Unmarshal(cfg))

	expected := factory.CreateDefaultConfig().(*Config)
	expected.Hosts = []confignet.TCPAddrConfig{
		{
			Endpoint: "cluster0.example.mongodb.net",
		},
	}
	expected.Scheme = "mongodb+srv"
	expected.Username = "otel"
	expected.Password = "${env:MONGO_PASSWORD}"
	expected.CollectionInterval = time.Minute

	require.Equal(t, expected, cfg)
}
