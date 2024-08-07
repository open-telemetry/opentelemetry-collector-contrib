// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tcpcheckreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/tcpcheckreceiver"

import (
	"testing"
	"time"

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
			cfg: &Config{
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
		},
		{
			desc: "invalid endpoint",
			cfg: &Config{
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
		},
		{
			cfg: &Config{
				},
				ControllerConfig: scraperhelper.NewDefaultControllerConfig(),
			},
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

