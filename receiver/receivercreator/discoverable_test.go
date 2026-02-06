// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package receivercreator

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateEndpointConfig(t *testing.T) {
	tests := []struct {
		name       string
		rawCfg     map[string]any
		endpoint   string
		discovered string
		expectErr  bool
	}{
		{
			name:       "full URL matching host",
			endpoint:   "http://test-endpoint",
			discovered: "test-endpoint",
			expectErr:  false,
		},
		{
			name:       "mismatched url",
			endpoint:   "http://evil-endpoint",
			discovered: "test-endpoint",
			expectErr:  true,
		},
		{
			name:       "dynamic endpoint reference",
			endpoint:   "http://`endpoint`:4317",
			discovered: "test-endpoint",
			expectErr:  true,
		},
		{
			name:       "bare endpoint",
			endpoint:   "test-endpoint",
			discovered: "test-endpoint",
			expectErr:  true,
		},
		{
			name:       "endpoint key missing",
			rawCfg:     map[string]any{"bad-key": "test"},
			discovered: "test-endpoint",
			expectErr:  false,
		},
		{
			name: "endpoint value is empty",
			rawCfg: map[string]any{
				"endpoint": "",
			},
			discovered: "test-endpoint",
			expectErr:  false,
		},
		{
			name: "endpoint value is an int",
			rawCfg: map[string]any{
				"endpoint": 123,
			},
			discovered: "test-endpoint",
			expectErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rawCfg := tt.rawCfg
			if rawCfg == nil {
				rawCfg = map[string]any{"endpoint": tt.endpoint}
			}
			err := ValidateEndpointConfig(rawCfg, tt.discovered)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err, "required no error")
			}
		})
	}
}
