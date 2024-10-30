// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tlscheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tlscheckreceiver"

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

func TestValidate(t *testing.T) {
	testCases := []struct {
		desc        string
		cfg         *Config
		expectedErr error
	}{
		{
			desc: "missing url",
			cfg: &Config{
				Targets:          []*targetConfig{},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: errMissingURL,
		},
		{
			desc: "invalid url",
			cfg: &Config{
				Targets: []*targetConfig{
					{
						URL: "invalid://endpoint:  12efg",
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: fmt.Errorf("%w: %s", errInvalidURL, `parse "invalid://endpoint:  12efg": invalid port ":  12efg" after host`),
		},
		{
			desc: "invalid config with multiple targets",
			cfg: &Config{
				Targets: []*targetConfig{
					{
						URL: "invalid://endpoint:  12efg",
					},
					{
						URL: "https://example.com",
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: fmt.Errorf("%w: %s", errInvalidURL, `parse "invalid://endpoint:  12efg": invalid port ":  12efg" after host`),
		},
		{
			desc: "missing scheme",
			cfg: &Config{
				Targets: []*targetConfig{
					{
						URL: "www.opentelemetry.io/docs",
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: fmt.Errorf("%w: %s", errInvalidURL, `parse "www.opentelemetry.io/docs": invalid URI for request`),
		},
		{
			desc: "valid config",
			cfg: &Config{
				Targets: []*targetConfig{
					{
						URL: "https://opentelemetry.io",
					},
					{
						URL: "https://opentelemetry.io:80/docs",
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: nil,
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
