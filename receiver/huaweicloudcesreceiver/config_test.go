// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package huaweicloudcesreceiver

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name          string
		config        Config
		expectedError string
	}{
		{
			name: "Valid config",
			config: Config{
				HuaweiSessionConfig: HuaweiSessionConfig{
					RegionName: "cn-north-1",
				},
				ProjectId: "my_project",
			},
			expectedError: "",
		},
		{
			name: "Missing region name",
			config: Config{
				ProjectId: "my_project",
			},
			expectedError: "region_name must be specified",
		},
		{
			name: "Missing project id",
			config: Config{
				HuaweiSessionConfig: HuaweiSessionConfig{
					RegionName: "cn-north-1",
				},
			},
			expectedError: "project_id must be specified",
		},
		{
			name: "Proxy user without proxy address",
			config: Config{
				HuaweiSessionConfig: HuaweiSessionConfig{
					RegionName: "cn-north-1",
					ProxyUser:  "user",
				},
				ProjectId: "my_project",
			},
			expectedError: "proxy_address must be specified if proxy_user or proxy_password is set",
		},
		{
			name: "Proxy password without proxy address",
			config: Config{
				HuaweiSessionConfig: HuaweiSessionConfig{
					RegionName:    "cn-north-1",
					ProxyPassword: "password",
				},
				ProjectId: "my_project",
			},
			expectedError: "proxy_address must be specified if proxy_user or proxy_password is set",
		},
		{
			name: "Proxy address with proxy user and password",
			config: Config{
				HuaweiSessionConfig: HuaweiSessionConfig{
					RegionName:    "cn-north-1",
					ProxyAddress:  "http://proxy.example.com",
					ProxyUser:     "user",
					ProxyPassword: "password",
				},
				ProjectId: "my_project",
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.expectedError)
			}
		})
	}
}
