// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottlfuncs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_SHA1(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected interface{}
		err      bool
	}{
		{
			name:     "string",
			value:    "hello world",
			expected: "2aae6c35c94fcfb415dbe95f408b9ce91ee846ed",
		},
		{
			name:     "empty string",
			value:    "",
			expected: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exprFunc, err := SHA1HashString[interface{}](&ottl.StandardStringGetter[interface{}]{
				Getter: func(context.Context, interface{}) (interface{}, error) {
					return tt.value, nil
				},
			})
			assert.NoError(t, err)
			result, err := exprFunc(nil, nil)
			if tt.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.expected, result)
		})
	}
}
