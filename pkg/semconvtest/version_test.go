// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//go:build integration

package semconvtest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateWeaverVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		wantErr bool
	}{
		{name: "minimum supported version passes", version: "v0.22.1", wantErr: false},
		{name: "older version rejected", version: "v0.21.2", wantErr: true},
		{name: "missing v prefix is normalized", version: "0.23.0", wantErr: false},
		{name: "missing v prefix on old version rejected", version: "0.21.0", wantErr: true},
		{name: "non-semver tag passed through", version: "sha-abc123", wantErr: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWeaverVersion(tt.version)
			if tt.wantErr {
				require.ErrorContains(t, err, "is not supported")
			} else {
				require.NoError(t, err)
			}
		})
	}
}
