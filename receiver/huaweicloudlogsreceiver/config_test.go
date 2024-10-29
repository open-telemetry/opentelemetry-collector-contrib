// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package huaweicloudlogsreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/receiver/scraperhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/huawei"
)

func TestConfigValidate(t *testing.T) {
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
				RegionID:  "cn-north-1",
				ProjectID: "my_project",
				GroupID:   "group-1",
				StreamID:  "stream-1",
			},
			expectedError: "",
		},
		{
			name: "Missing region name",
			config: Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: time.Hour,
				},
				ProjectID: "my_project",
			},
			expectedError: huawei.ErrMissingRegionID.Error(),
		},
		{
			name: "Missing project id",
			config: Config{
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: time.Hour,
				},
				RegionID: "cn-north-1",
			},
			expectedError: huawei.ErrMissingProjectID.Error(),
		},
		{
			name: "Proxy user without proxy address",
			config: Config{
				HuaweiSessionConfig: huawei.HuaweiSessionConfig{
					ProxyUser: "user",
				},
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: time.Hour,
				},
				RegionID:  "cn-north-1",
				ProjectID: "my_project",
				GroupID:   "group-1",
				StreamID:  "stream-1",
			},
			expectedError: huawei.ErrInvalidProxy.Error(),
		},
		{
			name: "Proxy password without proxy address",
			config: Config{
				HuaweiSessionConfig: huawei.HuaweiSessionConfig{
					ProxyPassword: "password",
				},
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: time.Hour,
				},
				RegionID:  "cn-north-1",
				ProjectID: "my_project",
				GroupID:   "group-1",
				StreamID:  "stream-1",
			},
			expectedError: huawei.ErrInvalidProxy.Error(),
		},
		{
			name: "Proxy address with proxy user and password",
			config: Config{
				HuaweiSessionConfig: huawei.HuaweiSessionConfig{
					ProxyAddress:  "http://proxy.example.com",
					ProxyUser:     "user",
					ProxyPassword: "password",
				},
				ControllerConfig: scraperhelper.ControllerConfig{
					CollectionInterval: time.Hour,
				},
				RegionID:  "cn-north-1",
				ProjectID: "my_project",
				GroupID:   "group-1",
				StreamID:  "stream-1",
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
