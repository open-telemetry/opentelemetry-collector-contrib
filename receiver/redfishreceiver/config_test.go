// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redfishreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/confmap"
)

func TestConfig_Validate(t *testing.T) {
	validResources := make([]Resource, 2)
	validResources[0] = ChassisResource
	validResources[1] = TemperaturesResource

	testcases := []struct {
		desc        string
		cfg         *Config
		expectedErr string
	}{
		{
			desc: "Bad Config - No Servers",
			cfg: &Config{
				Servers: []Server{},
			},
			expectedErr: "servers must not be empty",
		},
		{
			desc: "Bad Config - Bad BaseURL",
			cfg: &Config{
				Servers: []Server{
					{BaseURL: "htt//:website:55", User: "test-user", Pwd: "test-pwd", ComputerSystemID: "1", Resources: validResources},
				},
			},
			expectedErr: "invalid URI for request",
		},
		{
			desc: "Bad Config - No ComputerSystemID",
			cfg: &Config{
				Servers: []Server{
					{BaseURL: "http://localhost:3000", User: "test-user", Pwd: "test-pwd", Resources: validResources},
				},
			},
			expectedErr: "computer_system_id must not be empty",
		},
		{
			desc: "Bad Config - Bad Server Timeout",
			cfg: &Config{
				Servers: []Server{
					{BaseURL: "http://localhost:3000", Timeout: "abc", User: "test-user", Pwd: "test-pwd", ComputerSystemID: "1", Resources: validResources},
				},
			},
			expectedErr: "invalid server timeout",
		},
		{
			desc: "Bad Config - No Resources",
			cfg: &Config{
				Servers: []Server{
					{BaseURL: "http://localhost:3000", User: "test-user", Pwd: "test-pwd", ComputerSystemID: "1"},
				},
			},
			expectedErr: "resources must not be empty",
		},
		{
			desc: "Bad Config - TLS Cert Without Key",
			cfg: &Config{
				Servers: []Server{
					{BaseURL: "https://localhost:3000", TLS: configtls.ClientConfig{Config: configtls.Config{CertFile: "client.crt"}}, ComputerSystemID: "1", Resources: validResources},
				},
			},
			expectedErr: "TLS configuration must include both certificate and key",
		},
		{
			desc: "Bad Config - TLS Insecure",
			cfg: &Config{
				Servers: []Server{
					{BaseURL: "https://localhost:3000", TLS: configtls.ClientConfig{Insecure: true}, ComputerSystemID: "1", Resources: validResources},
				},
			},
			expectedErr: "tls::insecure is not supported",
		},
		{
			desc: "Valid Config - TLS Insecure Skip Verify",
			cfg: &Config{
				Servers: []Server{
					{BaseURL: "https://localhost:3000", TLS: configtls.ClientConfig{InsecureSkipVerify: true}, ComputerSystemID: "1", Resources: validResources},
				},
			},
		},
		{
			desc: "Valid Config - mTLS",
			cfg: &Config{
				Servers: []Server{
					{BaseURL: "https://localhost:3000", TLS: configtls.ClientConfig{Config: configtls.Config{CAFile: "ca.crt", CertFile: "client.crt", KeyFile: "client.key"}}, ComputerSystemID: "1", Resources: validResources},
				},
			},
		},
		{
			desc: "Valid Config",
			cfg: &Config{
				Servers: []Server{
					{BaseURL: "http://localhost:3000", Timeout: "60s", User: "test-user", Pwd: "test-pwd", ComputerSystemID: "1", Resources: validResources},
				},
			},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			cfg := createDefaultConfig().(*Config)
			cfg.Servers = tc.cfg.Servers
			err := confmap.Validate(cfg)
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
