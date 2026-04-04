// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package googlesecopsexporter

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func TestConfigValidate(t *testing.T) {
	testCases := []struct {
		desc        string
		config      *Config
		expectedErr string
	}{
		{
			desc:        "nil config",
			config:      nil,
			expectedErr: "config is nil",
		},
		{
			desc: "valid config",
			config: &Config{
				TimeoutConfig:    exporterhelper.NewDefaultTimeoutConfig(),
				QueueBatchConfig: configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
				BackOffConfig:    configretry.NewDefaultBackOffConfig(),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.config.Validate()
			if tc.expectedErr == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectedErr)
			}
		})
	}
}
