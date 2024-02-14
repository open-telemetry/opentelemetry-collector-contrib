// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package protocol

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/confmap"
)

func TestLoadParserConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfgMap  map[string]any
		want    Config
		wantErr bool
	}{
		{
			name:    "unknow_type",
			cfgMap:  map[string]any{"type": "unknown"},
			want:    Config{Type: "unknown"},
			wantErr: true,
		},
		{
			// Keep this test before the default_regex to ensure that the
			// default configuration is not being corrupted.
			name: "custom_delimiter",
			cfgMap: map[string]any{
				"type": "regex",
				"config": map[string]any{
					"rules": []any{map[string]any{"regexp": "(?<key_test>.*test)"}},
				},
			},
			want: Config{
				Type: "regex",
				Config: &RegexParserConfig{
					Rules: []*RegexRule{
						{Regexp: "(?<key_test>.*test)"},
					}},
			},
		},
		{
			name:   "default_regex",
			cfgMap: map[string]any{"type": "regex"},
			want: Config{
				Type:   "regex",
				Config: &RegexParserConfig{},
			},
		},
		{
			name:   "plaintext",
			cfgMap: map[string]any{"type": "plaintext"},
			want: Config{
				Type:   "plaintext",
				Config: &PlaintextConfig{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := confmap.NewFromStringMap(tt.cfgMap)

			got := Config{}
			err := got.Unmarshal(v)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}
