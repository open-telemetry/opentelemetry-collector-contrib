// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package icmpcheckreceiver

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/multierr"
)

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name               string
		cfg                *Config
		expectedUniqueErrs []error
		expectedCount      int
	}{
		{
			name:               "no targets",
			cfg:                &Config{},
			expectedUniqueErrs: []error{errMissingTarget},
			expectedCount:      1,
		},
		{
			name:               "single empty address",
			cfg:                &Config{Targets: []PingTarget{{}}},
			expectedUniqueErrs: []error{errMissingTargetHost},
			expectedCount:      1,
		},
		{
			name:               "multiple empty addresses",
			cfg:                &Config{Targets: []PingTarget{{Host: ""}, {Host: ""}}},
			expectedUniqueErrs: []error{errMissingTargetHost},
			expectedCount:      2,
		},
		{
			name:               "one valid one invalid",
			cfg:                &Config{Targets: []PingTarget{{Host: "1.1.1.1"}, {Host: ""}}},
			expectedUniqueErrs: []error{errMissingTargetHost},
			expectedCount:      1,
		},
		{
			name:               "all valid",
			cfg:                &Config{Targets: []PingTarget{{Host: "1.1.1.1"}, {Host: "example.com"}}},
			expectedUniqueErrs: []error{},
			expectedCount:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.expectedCount == 0 {
				require.NoError(t, err, "expected no error")
				return
			}
			require.Error(t, err, "expected error(s)")

			all := multierr.Errors(err)
			require.Len(t, all, tt.expectedCount, "unexpected number of collected errors: %v", all)

			// Check each expected unique error is present via errors.Is
			for _, expected := range tt.expectedUniqueErrs {
				require.ErrorIs(t, err, expected, "expected error not found")
			}

			// Also ensure no unexpected errors: each collected error must match one of expectedUniqueErrs
			for _, got := range all {
				found := false
				for _, expected := range tt.expectedUniqueErrs {
					if errors.Is(got, expected) {
						found = true
						break
					}
				}
				require.True(t, found, "unexpected error returned: %v", got)
			}
		})
	}
}
