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
		name       string
		priority   []string
		statements []string
		expected   string
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
			name:       "with unknown context",
			priority:   []string{"foo", "bar"},
			statements: []string{"set(span.foo, true) where span.bar == true"},
			expected:   "span",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inferrer := newPriorityContextInferrer(tt.priority)
			inferredContext, err := inferrer.infer(tt.statements)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, inferredContext)
		})
	}
}

func Test_NewPriorityContextInferrer_InvalidStatement(t *testing.T) {
	inferrer := newPriorityContextInferrer([]string{"foo"})
	statements := []string{"set(foo.field,"}
	_, err := inferrer.infer(statements)
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

	inferrer := defaultPriorityContextInferrer().(*priorityContextInferrer)
	require.NotNil(t, inferrer)

	for pri, ctx := range expectedPriority {
		require.Equal(t, pri, inferrer.contextPriority[ctx])
	}
}
