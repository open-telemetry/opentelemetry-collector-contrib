// Copyright 2019, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package protocol

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config"
)

func TestLoadParserConfig(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		cfg     Config
		want    Config
		wantErr bool
	}{
		{
			name:    "unknow_type",
			yaml:    `type: unknow`,
			cfg:     Config{Type: "unknown"},
			want:    Config{Type: "unknown"},
			wantErr: true,
		},
		{
			// Keep this test before the default_regex to ensure that the
			// default configuration is not being corrupted.
			name: "custom_delimiter",
			yaml: `
type: regex
config:
  rules:
    - regexp: "(?<key_test>.*test)"
`,
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
			name: "default_regex",
			yaml: `type: regex`,
			cfg:  Config{Type: "regex"},
			want: Config{
				Type:   "regex",
				Config: &RegexParserConfig{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := config.NewMapFromBuffer(strings.NewReader(tt.yaml))
			require.NoError(t, err)

			got := tt.cfg // Not strictly necessary but it makes easier to debug issues.
			err = LoadParserConfig(v, &got)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}
