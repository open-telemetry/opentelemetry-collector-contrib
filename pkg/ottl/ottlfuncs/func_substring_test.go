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

func Test_substring(t *testing.T) {
	tests := []struct {
		name     string
		target   ottl.StringGetter[interface{}]
		start    int64
		length   int64
		expected interface{}
	}{
		{
			name: "substring",
			target: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "123456789", nil
				},
			},
			start:    3,
			length:   3,
			expected: "456",
		},
		{
			name: "substring with result of total string",
			target: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "123456789", nil
				},
			},
			start:    0,
			length:   9,
			expected: "123456789",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := substring(tt.target, tt.start, tt.length)
			assert.NoError(t, err)
			result, err := exprFunc(nil, nil)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_substring_validation(t *testing.T) {
	tests := []struct {
		name   string
		target ottl.StringGetter[interface{}]
		start  int64
		length int64
	}{
		{
			name: "substring with result of empty string",
			target: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "123456789", nil
				},
			},
			start:  0,
			length: 0,
		},
		{
			name: "substring with invalid start index",
			target: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "123456789", nil
				},
			},
			start:  -1,
			length: 6,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := substring(tt.target, tt.start, tt.length)
			assert.Error(t, err)
		})
	}
}

func Test_substring_error(t *testing.T) {
	tests := []struct {
		name   string
		target ottl.StringGetter[interface{}]
		start  int64
		length int64
	}{
		{
			name: "substring empty string",
			target: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "", nil
				},
			},
			start:  3,
			length: 6,
		},
		{
			name: "substring with invalid length index",
			target: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return "123456789", nil
				},
			},
			start:  3,
			length: 20,
		},
		{
			name: "substring non-string",
			target: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return 123456789, nil
				},
			},
			start:  3,
			length: 6,
		},
		{
			name: "substring nil string",
			target: &ottl.StandardTypeGetter[interface{}, string]{
				Getter: func(ctx context.Context, tCtx interface{}) (interface{}, error) {
					return nil, nil
				},
			},
			start:  3,
			length: 6,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := substring(tt.target, tt.start, tt.length)
			assert.NoError(t, err)
			result, err := exprFunc(nil, nil)
			assert.Error(t, err)
			assert.Equal(t, nil, result)
		})
	}
}
