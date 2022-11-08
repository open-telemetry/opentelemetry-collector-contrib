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

func Test_concat(t *testing.T) {
	tests := []struct {
		name      string
		vals      []ottl.StandardGetSetter[interface{}]
		delimiter string
		expected  string
	}{
		{
			name: "concat strings",
			vals: []ottl.StandardGetSetter[interface{}]{
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return "hello", nil
					},
				},
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return "world", nil
					},
				},
			},
			delimiter: " ",
			expected:  "hello world",
		},
		{
			name: "nil",
			vals: []ottl.StandardGetSetter[interface{}]{
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return "hello", nil
					},
				},
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return nil, nil
					},
				},
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return "world", nil
					},
				},
			},
			delimiter: "",
			expected:  "hello<nil>world",
		},
		{
			name: "integers",
			vals: []ottl.StandardGetSetter[interface{}]{
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return "hello", nil
					},
				},
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return int64(1), nil
					},
				},
			},
			delimiter: "",
			expected:  "hello1",
		},
		{
			name: "floats",
			vals: []ottl.StandardGetSetter[interface{}]{
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return "hello", nil
					},
				},
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return 3.14159, nil
					},
				},
			},
			delimiter: "",
			expected:  "hello3.14159",
		},
		{
			name: "booleans",
			vals: []ottl.StandardGetSetter[interface{}]{
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return "hello", nil
					},
				},
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return true, nil
					},
				},
			},
			delimiter: " ",
			expected:  "hello true",
		},
		{
			name: "byte slices",
			vals: []ottl.StandardGetSetter[interface{}]{
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0e, 0xd2, 0xe6, 0x3c, 0xbe, 0x71, 0xf5, 0xa8}, nil
					},
				},
			},
			delimiter: "",
			expected:  "00000000000000000ed2e63cbe71f5a8",
		},
		{
			name: "non-byte slices",
			vals: []ottl.StandardGetSetter[interface{}]{
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 0}, nil
					},
				},
			},
			delimiter: "",
			expected:  "",
		},
		{
			name: "maps",
			vals: []ottl.StandardGetSetter[interface{}]{
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return map[string]string{"key": "value"}, nil
					},
				},
			},
			delimiter: "",
			expected:  "",
		},
		{
			name: "unprintable value in the middle",
			vals: []ottl.StandardGetSetter[interface{}]{
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return "hello", nil
					},
				},
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return map[string]string{"key": "value"}, nil
					},
				},
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return "world", nil
					},
				},
			},
			delimiter: "-",
			expected:  "hello--world",
		},
		{
			name: "empty string values",
			vals: []ottl.StandardGetSetter[interface{}]{
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return "", nil
					},
				},
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return "", nil
					},
				},
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return "", nil
					},
				},
			},
			delimiter: "__",
			expected:  "____",
		},
		{
			name: "single argument",
			vals: []ottl.StandardGetSetter[interface{}]{
				{
					Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
						return "hello", nil
					},
				},
			},
			delimiter: "-",
			expected:  "hello",
		},
		{
			name:      "no arguments",
			vals:      []ottl.StandardGetSetter[interface{}]{},
			delimiter: "-",
			expected:  "",
		},
		{
			name:      "no arguments with an empty delimiter",
			vals:      []ottl.StandardGetSetter[interface{}]{},
			delimiter: "",
			expected:  "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			getters := make([]ottl.Getter[interface{}], len(tt.vals))

			for i, val := range tt.vals {
				getters[i] = val
			}

			exprFunc, err := Concat(getters, tt.delimiter)
			assert.NoError(t, err)
			result, err := exprFunc(nil, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}
