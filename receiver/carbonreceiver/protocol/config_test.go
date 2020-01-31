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

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			// Keep this test before the default_delimiter to ensure that the
			// default configuration is not being corrupted.
			name: "custom_delimiter",
			yaml: `
type: delimiter
config:
  or_delimiter: o
`,
			cfg: Config{Type: "delimiter"},
			want: Config{
				Type: "delimiter",
				Config: &DelimiterParser{
					OrDemiliter: "o",
				},
			},
		},
		{
			name: "default_delimiter",
			yaml: `type: delimiter`,
			cfg:  Config{Type: "delimiter"},
			want: Config{
				Type:   "delimiter",
				Config: &DelimiterParser{},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := viper.New()
			v.SetConfigType("yaml")
			require.NoError(t, v.ReadConfig(strings.NewReader(tt.yaml)))

			got := tt.cfg // Not strictly necessary but it makes easier to debug issues.
			err := LoadParserConfig(v, &got)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, tt.wantErr, err != nil)
		})
	}
}
