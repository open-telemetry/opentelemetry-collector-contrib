// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_NewPriorityContextInferrer_Infer(t *testing.T) {
	tests := []struct {
		name          string
		priority      []string
		ignoreUnknown bool
		statements    []string
		expected      string
	}{
		{
			name:       "with priority and contexts",
			priority:   []string{"spanevent", "span", "resource"},
			statements: []string{"set(span.foo, resource.value) where spanevent.bar == true"},
			expected:   "spanevent",
		},
		{
			name:     "with multiple statements",
			priority: []string{"spanevent", "span", "resource"},
			statements: []string{
				"set(resource.foo, resource.value) where span.bar == true",
				"set(resource.foo, resource.value) where spanevent.bar == true",
			},
			expected: "spanevent",
		},
		{
			name:       "with no context",
			priority:   []string{"log", "resource"},
			statements: []string{"set(foo, true) where bar == true"},
			expected:   "",
		},
		{
			name:       "with empty priority",
			statements: []string{"set(foo.name, true) where bar.name == true"},
			expected:   "foo",
		},
		{
			name:          "with ignore unknown false",
			priority:      []string{"foo", "bar"},
			ignoreUnknown: false,
			statements:    []string{"set(span.foo, true) where span.bar == true"},
			expected:      "span",
		},
		{
			name:          "with ignore unknown true",
			priority:      []string{"foo", "bar"},
			ignoreUnknown: true,
			statements:    []string{"set(span.foo, true) where span.bar == true"},
			expected:      "",
		},
		{
			name:          "with ignore unknown true and mixed statement contexts",
			priority:      []string{"foo", "span"},
			ignoreUnknown: true,
			statements:    []string{"set(bar.foo, true) where span.bar == true"},
			expected:      "span",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inferrer := NewPriorityContextInferrer(tt.priority, tt.ignoreUnknown)
			inferredContext, err := inferrer.Infer(tt.statements)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, inferredContext)
		})
	}
}

func Test_NewPriorityContextInferrer_InvalidStatement(t *testing.T) {
	inferrer := NewPriorityContextInferrer([]string{"foo"}, false)
	statements := []string{"set(foo.field,"}
	_, err := inferrer.Infer(statements)
	require.ErrorContains(t, err, "unexpected token")
}

func Test_DefaultPriorityContextInferrer(t *testing.T) {
	expectedPriority := []string{
		"log",
		"metric",
		"datapoint",
		"spanevent",
		"span",
		"resource",
		"scope",
		"instrumentation_scope",
	}

	inferrer := DefaultPriorityContextInferrer().(*priorityContextInferrer)
	require.NotNil(t, inferrer)
	require.False(t, inferrer.ignoreUnknownContext)

	for pri, ctx := range expectedPriority {
		require.Equal(t, pri, inferrer.contextPriority[ctx])
	}
}
