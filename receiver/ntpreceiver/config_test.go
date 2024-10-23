// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ntpreceiver

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/receiver/scraperhelper"
)

func TestValidate(t *testing.T) {
	for _, tt := range []struct {
		name          string
		c             *Config
		errorExpected string
	}{
		{
			name: "no host",
			c: &Config{
				Version:          4,
				Endpoint:         "",
				ControllerConfig: scraperhelper.ControllerConfig{CollectionInterval: 45 * time.Minute},
			},
			errorExpected: "missing port in address",
		},
		{
			name: "no port",
			c: &Config{
				Version:          4,
				Endpoint:         "pool.ntp.org",
				ControllerConfig: scraperhelper.ControllerConfig{CollectionInterval: 45 * time.Minute},
			},
			errorExpected: "address pool.ntp.org: missing port in address",
		},
		{
			name: "valid",
			c: &Config{
				Version:          4,
				Endpoint:         "pool.ntp.org:123",
				ControllerConfig: scraperhelper.ControllerConfig{CollectionInterval: 45 * time.Minute},
			},
		},
		{
			name: "interval too small",
			c: &Config{
				Version:          4,
				Endpoint:         "pool.ntp.org:123",
				ControllerConfig: scraperhelper.ControllerConfig{CollectionInterval: 29 * time.Minute},
			},
			errorExpected: "collection interval 29m0s is less than minimum 30m",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.c.Validate()
			if tt.errorExpected == "" {
				require.NoError(t, err)
			} else {
				require.EqualError(t, err, tt.errorExpected)
			}
		})
	}
}
