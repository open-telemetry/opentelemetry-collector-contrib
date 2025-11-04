// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package systemdreceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/systemdreceiver/internal/metadata"
)

func TestValidate(t *testing.T) {
	testCases := []struct {
		desc        string
		cfg         *Config
		expectedErr error
	}{
		{
			desc: "invalid scope",
			cfg: &Config{
				Scope:                "unknown",
				Units:                []string{"*.service"},
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			},
			expectedErr: multierr.Combine(errInvalidScope),
		},
		{
			desc: "no units",
			cfg: &Config{
				Scope:                "system",
				Units:                []string{},
				ControllerConfig:     scraperhelper.NewDefaultControllerConfig(),
				MetricsBuilderConfig: metadata.DefaultMetricsBuilderConfig(),
			},
			expectedErr: multierr.Combine(errNoUnits),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			actualErr := tc.cfg.Validate()
			if tc.expectedErr != nil {
				require.EqualError(t, actualErr, tc.expectedErr.Error())
			} else {
				require.NoError(t, actualErr)
			}
		})
	}
}
