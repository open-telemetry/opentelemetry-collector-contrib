// Copyright  The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package keybuilder

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	t.Run("should create new keybuilder", func(t *testing.T) {
		assert.NotPanics(t, func() {
			assert.NotNil(t, New())
		})
	})
}

func TestMetricKeyBuilderAppend(t *testing.T) {
	tests := []struct {
		name   string
		args   [][]string
		result string
	}{
		{
			name:   "should output empty string when no string is appended",
			args:   [][]string{{}},
			result: "",
		},
		{
			name:   "should handle empty string",
			args:   [][]string{{"", "abc", "", "def"}},
			result: fmt.Sprintf("%sabc%s%sdef", Separator, Separator, Separator),
		},
		{
			name:   "should concat multiple append",
			args:   [][]string{{"abc", "def"}, {"", "hij"}},
			result: fmt.Sprintf("abc%sdef%s%shij", Separator, Separator, Separator),
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			kb := New()
			for _, arg := range tt.args {
				kb.Append(arg...)
			}
			assert.Equal(t, tt.result, kb.String())
		})
	}
}
