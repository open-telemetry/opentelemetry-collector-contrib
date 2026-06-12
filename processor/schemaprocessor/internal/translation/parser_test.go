// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translation

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		content      string
		wantFormat   SchemaFormat
		wantErr      bool
		wantErrIs    error
		wantPopulate func(t *testing.T, p *ParsedSchema)
	}{
		{
			name:       "v1.1",
			content:    LoadTranslationVersion(t, TranslationVersion190),
			wantFormat: SchemaFormatV1,
			wantPopulate: func(t *testing.T, p *ParsedSchema) {
				require.NotNil(t, p.V1)
				assert.NotEmpty(t, p.V1.FileFormat)
				assert.NotEmpty(t, p.V1.Versions)
			},
		},
		{
			name:       "v2 manifest",
			content:    LoadTranslationVersion(t, "v2_manifest_simple.yaml"),
			wantFormat: SchemaFormatV2Manifest,
			wantPopulate: func(t *testing.T, p *ParsedSchema) {
				require.NotNil(t, p.V2Manifest)
				assert.Equal(t, "https://example.com/schemas/2.0.0/resolved.yaml", p.V2Manifest.ResolvedRegistryURI)
			},
		},
		{
			name:       "v2 resolved",
			content:    LoadTranslationVersion(t, "v2_resolved_simple.yaml"),
			wantFormat: SchemaFormatV2Resolved,
			wantPopulate: func(t *testing.T, p *ParsedSchema) {
				require.NotNil(t, p.V2Resolved)
				assert.Len(t, p.V2Resolved.AttributeCatalog, 3)
			},
		},
		{
			name:      "v2 diff is unsupported",
			content:   LoadTranslationVersion(t, "v2_unsupported_diff.yaml"),
			wantErr:   true,
			wantErrIs: ErrUnsupportedSchemaFormat,
		},
		{
			name:    "missing file_format",
			content: "schema_url: https://example.com/1.0.0\n",
			wantErr: true,
		},
		{
			name:    "malformed yaml",
			content: ":\n  not yaml: [",
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Parse(tc.content)
			if tc.wantErr {
				require.Error(t, err)
				if tc.wantErrIs != nil {
					assert.ErrorIs(t, err, tc.wantErrIs)
				}
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantFormat, got.Format)
			if tc.wantPopulate != nil {
				tc.wantPopulate(t, got)
			}
		})
	}
}
