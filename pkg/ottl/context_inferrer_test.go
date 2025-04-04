// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ottl

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
)

var defaultDummyPriorityContextInferrerCandidate = &priorityContextInferrerCandidate{
	hasFunctionName: func(_ string) bool {
		return true
	},
	hasEnumSymbol: func(_ *EnumSymbol) bool {
		return true
	},
	getLowerContexts: func(_ string) []string {
		return nil
	},
}

func newDummyPriorityContextInferrerCandidate(hasFunctionName, hasEnumSymbol bool, lowerContexts []string) *priorityContextInferrerCandidate {
	return &priorityContextInferrerCandidate{
		hasFunctionName: func(_ string) bool {
			return hasFunctionName
		},
		hasEnumSymbol: func(_ *EnumSymbol) bool {
			return hasEnumSymbol
		},
		getLowerContexts: func(_ string) []string {
			return lowerContexts
		},
	}
}

func Test_NewPriorityContextInferrer_InferStatements(t *testing.T) {
	tests := []struct {
		name       string
		priority   []string
		candidates map[string]*priorityContextInferrerCandidate
		statements []string
		expected   string
	}{
		{
			name:     "with priority and statement context",
			priority: []string{"spanevent", "span", "resource"},
			candidates: map[string]*priorityContextInferrerCandidate{
				"spanevent": defaultDummyPriorityContextInferrerCandidate,
			},
			statements: []string{"set(span.foo, resource.value) where spanevent.bar == true"},
			expected:   "spanevent",
		},
		{
			name:     "with multiple statements and contexts",
			priority: []string{"spanevent", "span", "resource"},
			candidates: map[string]*priorityContextInferrerCandidate{
				"spanevent": defaultDummyPriorityContextInferrerCandidate,
			},
			statements: []string{
				"set(resource.foo, resource.value) where span.bar == true",
				"set(resource.foo, resource.value) where spanevent.bar == true",
			},
			expected: "spanevent",
		},
		{
			name:       "with no statements context",
			priority:   []string{"log", "resource"},
			candidates: map[string]*priorityContextInferrerCandidate{},
			statements: []string{"set(foo, true) where bar == true"},
			expected:   "",
		},
		{
			name: "with empty priority list",
			candidates: map[string]*priorityContextInferrerCandidate{
				"foo": defaultDummyPriorityContextInferrerCandidate,
			},
			statements: []string{"set(foo.name, true) where bar.name == true"},
			expected:   "foo",
		},
		{
			name:     "with non-prioritized statement context",
			priority: []string{"foo", "bar"},
			candidates: map[string]*priorityContextInferrerCandidate{
				"span": defaultDummyPriorityContextInferrerCandidate,
			},
			statements: []string{"set(span.foo, true) where span.bar == true"},
			expected:   "span",
		},
		{
			name:       "inferred path context with missing function",
			priority:   []string{"foo", "datapoint", "metric"},
			statements: []string{`set(metric.is_foo, true) where metric.name == "foo"`},
			candidates: map[string]*priorityContextInferrerCandidate{
				"metric":    newDummyPriorityContextInferrerCandidate(false, true, []string{"foo", "datapoint"}),
				"foo":       newDummyPriorityContextInferrerCandidate(false, true, []string{}),
				"datapoint": newDummyPriorityContextInferrerCandidate(true, true, []string{}),
			},
			expected: "datapoint",
		},
		{
			name:       "inferred path context with missing function and no qualified lower context",
			priority:   []string{"datapoint", "metric"},
			statements: []string{`set(metric.is_foo, true) where metric.name == "foo"`},
			candidates: map[string]*priorityContextInferrerCandidate{
				"metric":    newDummyPriorityContextInferrerCandidate(false, false, []string{"datapoint"}),
				"datapoint": newDummyPriorityContextInferrerCandidate(false, false, []string{}),
			},
			expected: "",
		},
		{
			name:       "inferred path context with missing function and no lower context",
			priority:   []string{"datapoint", "metric"},
			statements: []string{`set(metric.is_foo, true) where metric.name == "foo"`},
			candidates: map[string]*priorityContextInferrerCandidate{
				"metric": newDummyPriorityContextInferrerCandidate(false, true, []string{}),
			},
			expected: "",
		},
		{
			name:       "inferred path context with missing enum",
			priority:   []string{"foo", "bar"},
			statements: []string{`set(foo.name, FOO) where IsFoo() == true`},
			candidates: map[string]*priorityContextInferrerCandidate{
				"foo": newDummyPriorityContextInferrerCandidate(true, false, []string{"foo", "bar"}),
				"bar": newDummyPriorityContextInferrerCandidate(true, true, []string{}),
			},
			expected: "bar",
		},
		{
			name:       "unknown context candidate inferred from paths",
			priority:   []string{"unknown"},
			statements: []string{`set(unknown.count, 0)`},
			candidates: map[string]*priorityContextInferrerCandidate{},
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inferrer := newPriorityContextInferrer(
				componenttest.NewNopTelemetrySettings(),
				tt.candidates,
				withContextInferrerPriorities(tt.priority),
			)
			inferredContext, err := inferrer.inferFromStatements(tt.statements)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, inferredContext)
		})
	}
}

func Test_NewPriorityContextInferrer_InferConditions(t *testing.T) {
	tests := []struct {
		name       string
		priority   []string
		candidates map[string]*priorityContextInferrerCandidate
		conditions []string
		expected   string
	}{
		{
			name:     "with priority and statement context",
			priority: []string{"spanevent", "span", "resource"},
			candidates: map[string]*priorityContextInferrerCandidate{
				"spanevent": defaultDummyPriorityContextInferrerCandidate,
			},
			conditions: []string{"spanevent.bar == true"},
			expected:   "spanevent",
		},
		{
			name:     "with multiple conditions and contexts",
			priority: []string{"spanevent", "span", "resource"},
			candidates: map[string]*priorityContextInferrerCandidate{
				"spanevent": defaultDummyPriorityContextInferrerCandidate,
			},
			conditions: []string{
				"span.bar == true",
				"spanevent.bar == true",
			},
			expected: "spanevent",
		},
		{
			name:       "with no conditions context",
			priority:   []string{"log", "resource"},
			candidates: map[string]*priorityContextInferrerCandidate{},
			conditions: []string{"bar == true"},
			expected:   "",
		},
		{
			name: "with empty priority list",
			candidates: map[string]*priorityContextInferrerCandidate{
				"foo": defaultDummyPriorityContextInferrerCandidate,
			},
			conditions: []string{"foo.name == bar.name"},
			expected:   "foo",
		},
		{
			name:     "with non-prioritized statement context",
			priority: []string{"foo", "bar"},
			candidates: map[string]*priorityContextInferrerCandidate{
				"span": defaultDummyPriorityContextInferrerCandidate,
			},
			conditions: []string{"span.bar == true"},
			expected:   "span",
		},
		{
			name:       "inferred path context with missing function",
			priority:   []string{"foo", "datapoint", "metric"},
			conditions: []string{`HasFoo(metric.name) == metric.foo`},
			candidates: map[string]*priorityContextInferrerCandidate{
				"metric":    newDummyPriorityContextInferrerCandidate(false, true, []string{"foo", "datapoint"}),
				"foo":       newDummyPriorityContextInferrerCandidate(false, true, []string{}),
				"datapoint": newDummyPriorityContextInferrerCandidate(true, true, []string{}),
			},
			expected: "datapoint",
		},
		{
			name:       "inferred path context with missing function and no qualified lower context",
			priority:   []string{"datapoint", "metric"},
			conditions: []string{`DummyFunc("foo") == "hello"`},
			candidates: map[string]*priorityContextInferrerCandidate{
				"metric":    newDummyPriorityContextInferrerCandidate(false, false, []string{"datapoint"}),
				"datapoint": newDummyPriorityContextInferrerCandidate(false, false, []string{}),
			},
			expected: "",
		},
		{
			name:       "inferred path context with missing function and no lower context",
			priority:   []string{"datapoint", "metric"},
			conditions: []string{`is_name == "foo"`},
			candidates: map[string]*priorityContextInferrerCandidate{
				"metric": newDummyPriorityContextInferrerCandidate(false, true, []string{}),
			},
			expected: "",
		},
		{
			name:       "inferred path context with missing enum",
			priority:   []string{"foo", "bar"},
			conditions: []string{`foo.name == FOO`},
			candidates: map[string]*priorityContextInferrerCandidate{
				"foo": newDummyPriorityContextInferrerCandidate(true, false, []string{"foo", "bar"}),
				"bar": newDummyPriorityContextInferrerCandidate(true, true, []string{}),
			},
			expected: "bar",
		},
		{
			name:       "unknown context candidate inferred from paths",
			priority:   []string{"unknown"},
			conditions: []string{`count == 0`},
			candidates: map[string]*priorityContextInferrerCandidate{},
			expected:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inferrer := newPriorityContextInferrer(
				componenttest.NewNopTelemetrySettings(),
				tt.candidates,
				withContextInferrerPriorities(tt.priority),
			)
			inferredContext, err := inferrer.inferFromConditions(tt.conditions)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, inferredContext)
		})
	}
}

func Test_NewPriorityContextInferrer_InvalidStatement(t *testing.T) {
	inferrer := newPriorityContextInferrer(componenttest.NewNopTelemetrySettings(), map[string]*priorityContextInferrerCandidate{})
	statements := []string{"set(foo.field,"}
	_, err := inferrer.inferFromStatements(statements)
	require.ErrorContains(t, err, "unexpected token")
}

func Test_DefaultPriorityContextInferrer(t *testing.T) {
	expectedPriority := []string{
		"log",
		"datapoint",
		"metric",
		"spanevent",
		"span",
		"resource",
		"scope",
		"instrumentation_scope",
	}

	inferrer := newPriorityContextInferrer(componenttest.NewNopTelemetrySettings(), map[string]*priorityContextInferrerCandidate{}).(*priorityContextInferrer)
	require.NotNil(t, inferrer)

	for pri, ctx := range expectedPriority {
		require.Equal(t, pri, inferrer.contextPriority[ctx])
	}
}
