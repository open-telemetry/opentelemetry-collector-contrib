// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package lookupsource

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFileSourceConfig_Validate(t *testing.T) {
	tests := []struct {
		name        string
		cfg         FileSourceConfig
		expectedErr string
	}{
		{
			name: "valid config with positive reload interval",
			cfg: FileSourceConfig{
				Path:           "/etc/data.csv",
				ReloadInterval: 5 * time.Minute,
			},
			expectedErr: "",
		},
		{
			name: "valid config with zero reload interval (disabled)",
			cfg: FileSourceConfig{
				Path:           "/etc/data.csv",
				ReloadInterval: 0,
			},
			expectedErr: "",
		},
		{
			name: "missing path",
			cfg: FileSourceConfig{
				Path:           "",
				ReloadInterval: 10 * time.Second,
			},
			expectedErr: "path is required",
		},
		{
			name: "negative reload interval",
			cfg: FileSourceConfig{
				Path:           "/etc/data.csv",
				ReloadInterval: -1 * time.Second,
			},
			expectedErr: "reload_interval must not be negative",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Equal(t, tt.expectedErr, err.Error())
			}
		})
	}
}
