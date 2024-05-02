// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package docker // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/docker"

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPIVersion(t *testing.T) {
	for _, test := range []struct {
		input           string
		expectedVersion string
		expectedRep     string
		expectedError   string
	}{
		{
			input:           "1.2",
			expectedVersion: MustNewAPIVersion("1.2"),
			expectedRep:     "1.2",
		},
		{
			input:           "1.40",
			expectedVersion: MustNewAPIVersion("1.40"),
			expectedRep:     "1.40",
		},
		{
			input:           "10",
			expectedVersion: MustNewAPIVersion("10.0"),
			expectedRep:     "10.0",
		},
		{
			input:           "0",
			expectedVersion: MustNewAPIVersion("0.0"),
			expectedRep:     "0.0",
		},
		{
			input:           ".400",
			expectedVersion: MustNewAPIVersion("0.400"),
			expectedRep:     "0.400",
		},
		{
			input:           "00000.400",
			expectedVersion: MustNewAPIVersion("0.400"),
			expectedRep:     "0.400",
		},
		{
			input:         "0.1.",
			expectedError: `invalid version "0.1."`,
		},
		{
			input:         "0.1.2.3",
			expectedError: `invalid version "0.1.2.3"`,
		},
		{
			input:         "",
			expectedError: `invalid version ""`,
		},
		{
			input:         "...",
			expectedError: `invalid version "..."`,
		},
	} {
		t.Run(test.input, func(t *testing.T) {
			version, err := NewAPIVersion(test.input)
			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
				assert.Empty(t, version)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test.expectedVersion, version)
			require.Equal(t, test.expectedRep, version)
			require.Equal(t, test.expectedRep, version)
		})
	}
}
