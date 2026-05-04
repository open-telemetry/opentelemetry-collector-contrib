// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package genainormalizerprocessor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr string
	}{
		{
			name: "valid",
			cfg: Config{
				Sources: map[SourceName]Source{
					SourceOpenInference: {RemoveOriginals: true},
					SourceOpenLLMetry:   {},
				},
			},
		},
		{
			name:    "no sources",
			cfg:     Config{Sources: map[SourceName]Source{}},
			wantErr: "at least one source",
		},
		{
			name: "unknown source",
			cfg: Config{
				Sources: map[SourceName]Source{"bogus": {}},
			},
			wantErr: `unknown source "bogus"`,
		},
		{
			name: "custom_mappings empty source attr",
			cfg: Config{
				Sources: map[SourceName]Source{
					SourceOpenInference: {
						CustomMappings: map[string]string{"": "gen_ai.request.model"},
					},
				},
			},
			wantErr: "source attribute name must be non-empty",
		},
		{
			name: "custom_mappings empty target",
			cfg: Config{
				Sources: map[SourceName]Source{
					SourceOpenInference: {
						CustomMappings: map[string]string{"my_vendor.model": ""},
					},
				},
			},
			wantErr: `target for "my_vendor.model" must be non-empty`,
		},
		{
			name: "custom_mappings identity",
			cfg: Config{
				Sources: map[SourceName]Source{
					SourceOpenInference: {
						CustomMappings: map[string]string{"gen_ai.request.model": "gen_ai.request.model"},
					},
				},
			},
			wantErr: "source and target are identical",
		},
		{
			name: "valid custom_mappings",
			cfg: Config{
				Sources: map[SourceName]Source{
					SourceOpenInference: {
						CustomMappings: map[string]string{"my_vendor.model": "gen_ai.request.model"},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tt.wantErr)
			}
		})
	}
}
