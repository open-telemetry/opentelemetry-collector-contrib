// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package addon

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseID(t *testing.T) {
	tests := []struct {
		in            string
		expected      ID
		expectedError string
	}{
		{in: "file:geoip", expected: ID{Type: "file", Name: "geoip"}},
		{in: "jmx:tomcat", expected: ID{Type: "jmx", Name: "tomcat"}},
		// The component type grammar allows "_", which confmap schemes do not.
		{in: "custom_rules:base", expected: ID{Type: "custom_rules", Name: "base"}},
		// Only the first ":" separates; the name is opaque.
		{in: "file:dir/geo:2026.10", expected: ID{Type: "file", Name: "dir/geo:2026.10"}},
		{in: "geoip", expectedError: `must have the form "<type>:<name>"`},
		{in: "", expectedError: `must have the form "<type>:<name>"`},
		{in: ":geoip", expectedError: "invalid addon type"},
		{in: "file:", expectedError: "must not be empty"},
		{in: "1file:x", expectedError: "invalid addon type"},
		{in: "custom-rules:x", expectedError: "invalid addon type"},
		{in: "fi le:x", expectedError: "invalid addon type"},
		{in: strings.Repeat("a", 64) + ":x", expectedError: "invalid addon type"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			id, err := ParseID(tt.in)
			if tt.expectedError != "" {
				require.ErrorContains(t, err, tt.expectedError)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, id)
			// String must round-trip, since it is the package name and config key.
			assert.Equal(t, tt.in, id.String())
		})
	}
}
