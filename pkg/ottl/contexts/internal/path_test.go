// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
)

func Test_isPathToContextRoot(t *testing.T) {
	tests := []struct {
		name     string
		context  string
		path     ottl.Path[any]
		expected bool
	}{
		{
			name:     "with context and empty path",
			context:  "resource",
			expected: true,
			path: &TestPath[any]{
				C: "resource",
				N: "",
			},
		},
		{
			name:     "with context and next path",
			context:  "resource",
			expected: false,
			path: &TestPath[any]{
				C: "resource",
				N: "",
				NextPath: &TestPath[any]{
					N: "foo",
				},
			},
		},
		{
			name:     "with context and path keys",
			context:  "resource",
			expected: false,
			path: &TestPath[any]{
				C: "resource",
				N: "",
				KeySlice: []ottl.Key[any]{
					&TestKey[any]{},
				},
			},
		},
		{
			name:     "with both context and path name",
			context:  "resource",
			expected: false,
			path: &TestPath[any]{
				C: "resource",
				N: "resource",
			},
		},
		{
			name:     "with nil path",
			context:  "resource",
			expected: true,
			path:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, isPathToContextRoot(tt.path, tt.context))
		})
	}
}
