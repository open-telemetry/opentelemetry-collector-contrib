// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package verifier

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/open-telemetry/opentelemetry-collector-contrib/cmd/opampsupervisor/supervisor/config"
)

func TestNewVerifier(t *testing.T) {
	testCases := []struct {
		name        string
		verifier    config.Verifier
		expectType  string
		expectedErr string
	}{
		{
			name:       "none verifier",
			verifier:   config.Verifier{Type: config.VerifierTypeNone},
			expectType: config.VerifierTypeNone,
		},
		{
			name:        "unsupported verifier type",
			verifier:    config.Verifier{Type: "cosign"},
			expectedErr: "unsupported verifier type",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			v, err := NewVerifier(tc.verifier)
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
				assert.Nil(t, v)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, v)
			assert.Equal(t, tc.expectType, v.Type())
		})
	}
}

func TestNoneVerifierVerify(t *testing.T) {
	v := &noneVerifier{}
	assert.NoError(t, v.Verify([]byte("package"), []byte("signature")))
	assert.Equal(t, config.VerifierTypeNone, v.Type())
}
