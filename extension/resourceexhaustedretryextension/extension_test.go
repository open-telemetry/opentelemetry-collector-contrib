// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package resourceexhaustedretryextension

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name: "zero values valid",
			cfg:  Config{},
		},
		{
			name: "positive values valid",
			cfg:  Config{RetryDelay: 2 * time.Second, Jitter: 3 * time.Second},
		},
		{
			name:    "negative retry_delay",
			cfg:     Config{RetryDelay: -1 * time.Second},
			wantErr: "retry_delay must be non-negative",
		},
		{
			name:    "negative jitter",
			cfg:     Config{Jitter: -1 * time.Second},
			wantErr: "jitter must be non-negative",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
