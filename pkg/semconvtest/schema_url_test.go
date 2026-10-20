// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package semconvtest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeSchemaURL(t *testing.T) {
	tests := []struct {
		name        string
		schemaURL   string
		expectedReg string
		expectedErr string
	}{
		{
			name:        "standard schema URL",
			schemaURL:   "https://opentelemetry.io/schemas/1.27.0",
			expectedReg: "https://github.com/open-telemetry/semantic-conventions@v1.27.0[model]",
		},
		{
			name:        "standard schema URL older version",
			schemaURL:   "https://opentelemetry.io/schemas/1.21.0",
			expectedReg: "https://github.com/open-telemetry/semantic-conventions@v1.21.0[model]",
		},
		{
			name:        "trailing slash trimmed",
			schemaURL:   "https://opentelemetry.io/schemas/1.28.0/",
			expectedReg: "https://github.com/open-telemetry/semantic-conventions@v1.28.0[model]",
		},
		{
			name:        "v-prefixed version in URL",
			schemaURL:   "https://opentelemetry.io/schemas/v1.28.0",
			expectedReg: "https://github.com/open-telemetry/semantic-conventions@v1.28.0[model]",
		},
		{
			name:        "empty URL",
			schemaURL:   "",
			expectedErr: "schema URL must not be empty",
		},
		{
			name:        "unsupported scheme or domain",
			schemaURL:   "http://opentelemetry.io/schemas/1.27.0",
			expectedErr: "unsupported schema URL",
		},
		{
			name:        "custom third-party domain",
			schemaURL:   "https://example.com/schemas/1.0.0",
			expectedErr: "unsupported schema URL",
		},
		{
			name:        "invalid semver version",
			schemaURL:   "https://opentelemetry.io/schemas/not-a-version",
			expectedErr: "invalid semver version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg, err := normalizeSchemaURL(tt.schemaURL)
			if tt.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.expectedReg, reg)
			}
		})
	}
}
