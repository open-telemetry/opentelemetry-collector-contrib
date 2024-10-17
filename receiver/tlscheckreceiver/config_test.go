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
			expectedErr: ErrMissingTargets,
		},
		{
			desc: "invalid host",
			cfg: &Config{
				Targets: []*targetConfig{
					{
						Host: "endpoint:  12efg",
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: fmt.Errorf("%w: %s", errInvalidHost, "provided port is not a number:   12efg"),
		},
		{
			desc: "invalid config with multiple targets",
			cfg: &Config{
				Targets: []*targetConfig{
					{
						Host: "endpoint:  12efg",
					},
					{
						Host: "https://example.com:80",
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: fmt.Errorf("%w: %s", errInvalidHost, `provided port is not a number:   12efg; host contains a scheme, which is not allowed: https://example.com:80`),
		},
		{
			desc: "port out of range",
			cfg: &Config{
				Targets: []*targetConfig{
					{
						Host: "www.opentelemetry.io:67000",
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: fmt.Errorf("%w: %s", errInvalidHost, `provided port is out of valid range (1-65535): 67000`),
		},
		{
			desc: "missing scheme",
			cfg: &Config{
				Targets: []*targetConfig{
					{
						Host: "www.opentelemetry.io/docs",
					},
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
			expectedErr: fmt.Errorf("%w: %s", errInvalidHost, `address www.opentelemetry.io/docs: missing port in address`),
		},
		{
			desc: "valid config",
			cfg: &Config{
				Targets: []*targetConfig{
					{
						Host: "opentelemetry.io:443",
					},
					{
						Host: "opentelemetry.io:8080",
					},
					{
						Host: "111.222.33.44:10000",
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
