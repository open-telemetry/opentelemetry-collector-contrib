// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pebbletailstorageextension

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name: "valid unlimited",
			cfg: Config{
				Directory: "test-storage",
			},
		},
		{
			name: "valid bounded config",
			cfg: Config{
				Directory:         "test-storage",
				MaxStorageSizeMiB: 1,
			},
		},
		{
			name:    "missing directory",
			cfg:     Config{},
			wantErr: "directory must be set",
		},
		{
			name: "negative size",
			cfg: Config{
				Directory:         "test-storage",
				MaxStorageSizeMiB: -1,
			},
			wantErr: "max_storage_size_mib",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}
