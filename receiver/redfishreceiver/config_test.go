// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package redfishreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	validResources := make([]Resource, 2)
	validResources[0] = ChassisResource
	validResources[1] = TemperaturesResource

	testcases := []struct {
		desc        string
		cfg         *Config
		expectError bool
	}{
		{
			desc: "Bad Config - No Servers",
			cfg: &Config{
				Servers: []Server{},
			},
			expectError: true,
		},
		{
			desc: "Bad Config - Bad BaseURL",
			cfg: &Config{
				Servers: []Server{
					{BaseURL: "htt//:website:55", User: "test-user", Pwd: "test-pwd", ComputerSystemID: "1", Resources: validResources},
				},
			},
			expectError: true,
		},
		{
			desc: "Bad Config - No ComputerSystemID",
			cfg: &Config{
				Servers: []Server{
					{BaseURL: "http://localhost:3000", User: "test-user", Pwd: "test-pwd", Resources: validResources},
				},
			},
			expectError: true,
		},
		{
			desc: "Bad Config - Bad Server Timeout",
			cfg: &Config{
				Servers: []Server{
					{BaseURL: "http://localhost:3000", Timeout: "abc", User: "test-user", Pwd: "test-pwd", ComputerSystemID: "1", Resources: validResources},
				},
			},
			expectError: true,
		},
		{
			desc: "Bad Config - No Resources",
			cfg: &Config{
				Servers: []Server{
					{BaseURL: "http://localhost:3000", Timeout: "abc", User: "test-user", Pwd: "test-pwd", ComputerSystemID: "1"},
				},
			},
			expectError: true,
		},
		{
			desc: "Valid Config",
			cfg: &Config{
				Servers: []Server{
					{BaseURL: "http://localhost:3000", Timeout: "60s", User: "test-user", Pwd: "test-pwd", ComputerSystemID: "1", Resources: validResources},
				},
			},
			expectError: false,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
