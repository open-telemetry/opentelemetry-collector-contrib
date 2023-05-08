// Copyright The OpenTelemetry Authors
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

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_split(t *testing.T) {
	tests := []struct {
		name      string
		target    ottl.StringGetter[interface{}]
		delimiter string
		expected  interface{}
	}{
		{
			name: "split string",
			target: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "A|B|C", nil
				},
			},
			delimiter: "|",
			expected:  []string{"A", "B", "C"},
		},
		{
			name: "split empty string",
			target: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "", nil
				},
			},
			delimiter: "|",
			expected:  []string{""},
		},
		{
			name: "split empty delimiter",
			target: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "A|B|C", nil
				},
			},
			delimiter: "",
			expected:  []string{"A", "|", "B", "|", "C"},
		},
		{
			name: "split empty string and empty delimiter",
			target: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "", nil
				},
			},
			delimiter: "",
			expected:  []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc := split(tt.target, tt.delimiter)
			result, err := exprFunc(nil, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_Split_Error(t *testing.T) {
	target := &ottl.StandardTypeGetter[interface{}, string]{
		Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
			return 1, nil
		},
	}
	exprFunc := split[interface{}](target, ",")
	_, err := exprFunc(context.Background(), nil)
	assert.Error(t, err)
}
