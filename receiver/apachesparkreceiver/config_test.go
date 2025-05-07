// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package apachesparkreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/apachesparkreceiver"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
)

func TestValidate(t *testing.T) {
	clientConfig := confighttp.NewDefaultClientConfig()
	clientConfig.Endpoint = "invalid://endpoint  12efg"
	testCases := []struct {
		desc        string
		cfg         *Config
		expectedErr error
	}{
		{
			desc: "default config",
			cfg: &Config{
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: nil,
		},
		{
			desc: "invalid endpoint",
			cfg: &Config{
				ClientConfig:     clientConfig,
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errInvalidEndpoint,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			actualErr := tc.cfg.Validate()
			if tc.expectedErr != nil {
				require.ErrorContains(t, actualErr, tc.expectedErr.Error())
			} else {
				require.NoError(t, actualErr)
			}
		})
	}
}

func TestApplicationLimitConfig(t *testing.T) {
	cfg := &Config{}
	// Default should be zero (unlimited)
	assert.Equal(t, 0, cfg.Limit)

	cfg.Limit = 2
	assert.Equal(t, 2, cfg.Limit)
}

func TestStartTimeEpochLimitConfig(t *testing.T) {
	cfg := &Config{}
	// Default should be zero
	assert.Equal(t, int64(0), cfg.StartTimeEpochLimit)

	// Set a specific value
	cfg.StartTimeEpochLimit = 1680000000
	assert.Equal(t, int64(1680000000), cfg.StartTimeEpochLimit)
}
