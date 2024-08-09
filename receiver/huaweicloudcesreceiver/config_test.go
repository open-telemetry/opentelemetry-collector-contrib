// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package huaweicloudcesreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
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
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: time.Hour,
				},
				HuaweiSessionConfig: HuaweiSessionConfig{
					RegionName: "cn-north-1",
				},
				ProjectId: "my_project",
				Period:    300,
				Filter:    "min",
			},
			expectedError: "",
		},
		{
			name: "Invalid Period",
			config: Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: time.Hour,
				},
				HuaweiSessionConfig: HuaweiSessionConfig{
					RegionName: "cn-north-1",
				},
				ProjectId: "my_project",
				Period:    100,
				Filter:    "min",
			},
			expectedError: "invalid period",
		},
		{
			name: "Invalid Filter",
			config: Config{
				HuaweiSessionConfig: HuaweiSessionConfig{
					RegionName: "cn-north-1",
				},
				ProjectId: "my_project",
				Period:    300,
				Filter:    "invalid",
			},
			expectedError: "invalid filter",
		},
		{
			name: "Missing region name",
			config: Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: time.Hour,
				},
				ProjectId: "my_project",
				Period:    300,
				Filter:    "min",
			},
			expectedError: errMissingRegionName.Error(),
		},
		{
			name: "Missing project id",
			config: Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: time.Hour,
				},
				HuaweiSessionConfig: HuaweiSessionConfig{
					RegionName: "cn-north-1",
				},
				Period: 300,
				Filter: "min",
			},
			expectedError: errMissingProjectID.Error(),
		},
		{
			name: "Proxy user without proxy address",
			config: Config{
				HuaweiSessionConfig: HuaweiSessionConfig{
					RegionName: "cn-north-1",
					ProxyUser:  "user",
				},
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: time.Hour,
				},
				ProjectId: "my_project",
				Period:    300,
				Filter:    "min",
			},
			expectedError: errInvalidProxy.Error(),
		},
		{
			name: "Proxy password without proxy address",
			config: Config{
				HuaweiSessionConfig: HuaweiSessionConfig{
					RegionName:    "cn-north-1",
					ProxyPassword: "password",
				},
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: time.Hour,
				},
				ProjectId: "my_project",
				Period:    300,
				Filter:    "min",
			},
			expectedError: errInvalidProxy.Error(),
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
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: time.Hour,
				},
				ProjectId: "my_project",
				Period:    300,
				Filter:    "min",
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
				assert.ErrorContains(t, err, tt.expectedError)
			}
		})
	}
}
