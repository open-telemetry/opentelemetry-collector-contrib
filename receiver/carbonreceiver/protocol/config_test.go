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
		cfgMap  map[string]interface{}
		cfg     Config
		want    Config
		wantErr bool
	}{
		{
			name:    "unknow_type",
			cfgMap:  map[string]interface{}{"type": "unknow"},
			cfg:     Config{Type: "unknown"},
			want:    Config{Type: "unknown"},
			wantErr: true,
		},
		{
			// Keep this test before the default_regex to ensure that the
			// default configuration is not being corrupted.
			name: "custom_delimiter",
			cfgMap: map[string]interface{}{
				"type": "regex",
				"config": map[string]interface{}{
					"rules": []interface{}{map[string]interface{}{"regexp": "(?<key_test>.*test)"}},
				},
			},
			cfg: Config{Type: "regex"},
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
			cfgMap: map[string]interface{}{"type": "regex"},
			cfg:    Config{Type: "regex"},
			want: Config{
				Type:   "regex",
				Config: &RegexParserConfig{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := confmap.NewFromStringMap(tt.cfgMap)

			got := tt.cfg // Not strictly necessary but it makes easier to debug issues.
			err := LoadParserConfig(v, &got)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}
