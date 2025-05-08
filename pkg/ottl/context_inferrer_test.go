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
		err        string
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
			err: `inferred context "metric" does not support the function "set"`,
		},
		{
			name:       "inferred path context with missing function and no lower context",
			priority:   []string{"datapoint", "metric"},
			statements: []string{`set(metric.is_foo, true) where metric.name == "foo"`},
			candidates: map[string]*priorityContextInferrerCandidate{
				"metric": newDummyPriorityContextInferrerCandidate(false, true, []string{}),
			},
			err: `inferred context "metric" does not support the function "set"`,
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
			err:        `inferred context "unknown" is not a valid candidate`,
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
			if tt.err != "" {
				require.ErrorContains(t, err, tt.err)
				return
			}

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
	require.ErrorContains(t, err, "statement has invalid syntax")
}

func Test_NewPriorityContextInferrer_InvalidCondition(t *testing.T) {
	inferrer := newPriorityContextInferrer(componenttest.NewNopTelemetrySettings(), map[string]*priorityContextInferrerCandidate{})
	conditions := []string{"foo.field,"}
	_, err := inferrer.inferFromConditions(conditions)
	require.ErrorContains(t, err, "condition has invalid syntax")
}

func Test_NewPriorityContextInferrer_DefaultPriorityList(t *testing.T) {
	expectedPriority := []string{
		"log",
		"datapoint",
		"metric",
		"spanevent",
		"span",
		"profile",
		"scope",
		"instrumentation_scope",
		"resource",
	}

	inferrer := newPriorityContextInferrer(componenttest.NewNopTelemetrySettings(), map[string]*priorityContextInferrerCandidate{}).(*priorityContextInferrer)
	require.NotNil(t, inferrer)

	for pri, ctx := range expectedPriority {
		require.Equal(t, pri, inferrer.contextPriority[ctx])
	}
}

func Test_NewPriorityContextInferrer_InferStatements_DefaultContextsOrder(t *testing.T) {
	inferrer := newPriorityContextInferrer(componenttest.NewNopTelemetrySettings(), map[string]*priorityContextInferrerCandidate{
		"log":                   newDummyPriorityContextInferrerCandidate(true, true, []string{"scope", "instrumentation_scope", "resource"}),
		"metric":                newDummyPriorityContextInferrerCandidate(true, true, []string{"datapoint", "scope", "instrumentation_scope", "resource"}),
		"datapoint":             newDummyPriorityContextInferrerCandidate(true, true, []string{"scope", "instrumentation_scope", "resource"}),
		"span":                  newDummyPriorityContextInferrerCandidate(true, true, []string{"spanevent", "scope", "instrumentation_scope", "resource"}),
		"spanevent":             newDummyPriorityContextInferrerCandidate(true, true, []string{"scope", "instrumentation_scope", "resource"}),
		"profile":               newDummyPriorityContextInferrerCandidate(true, true, []string{"profile", "scope", "instrumentation_scope", "resource"}),
		"scope":                 newDummyPriorityContextInferrerCandidate(true, true, []string{"resource"}),
		"instrumentation_scope": newDummyPriorityContextInferrerCandidate(true, true, []string{"resource"}),
		"resource":              newDummyPriorityContextInferrerCandidate(true, true, []string{}),
	})

	tests := []struct {
		name      string
		statement string
		expected  string
	}{
		{
			name:      "log,instrumentation_scope,resource",
			statement: `set(log.attributes["foo"], true) where instrumentation_scope.attributes["foo"] == resource.attributes["foo"]`,
			expected:  "log",
		},
		{
			name:      "log,scope,resource",
			statement: `set(log.attributes["foo"], true) where scope.attributes["foo"] == resource.attributes["foo"]`,
			expected:  "log",
		},
		{
			name:      "instrumentation_scope,resource",
			statement: `set(instrumentation_scope.attributes["foo"], true) where resource.attributes["foo"] != nil`,
			expected:  "instrumentation_scope",
		},
		{
			name:      "scope,resource",
			statement: `set(scope.attributes["foo"], true) where resource.attributes["foo"] != nil`,
			expected:  "scope",
		},
		{
			name:      "metric,instrumentation_scope,resource",
			statement: `set(metric.name, "foo") where instrumentation_scope.name != nil and resource.attributes["foo"] != nil`,
			expected:  "metric",
		},
		{
			name:      "metric,scope,resource",
			statement: `set(metric.name, "foo") where scope.name != nil and resource.attributes["foo"] != nil`,
			expected:  "metric",
		},
		{
			name:      "datapoint,metric,instrumentation_scope,resource",
			statement: `set(metric.name, "foo") where datapoint.double_value > 0 and instrumentation_scope.name != nil and resource.attributes["foo"] != nil`,
			expected:  "datapoint",
		},
		{
			name:      "datapoint,metric,scope,resource",
			statement: `set(metric.name, "foo") where datapoint.double_value > 0 and scope.name != nil and resource.attributes["foo"] != nil`,
			expected:  "datapoint",
		},
		{
			name:      "span,instrumentation_scope,resource",
			statement: `set(span.name, "foo") where instrumentation_scope.name != nil and resource.attributes["foo"] != nil`,
			expected:  "span",
		},
		{
			name:      "span,scope,resource",
			statement: `set(span.name, "foo") where scope.name != nil and resource.attributes["foo"] != nil`,
			expected:  "span",
		},
		{
			name:      "spanevent,span,instrumentation_scope,resource",
			statement: `set(span.name, "foo") where spanevent.name != nil and instrumentation_scope.name != nil and resource.attributes["foo"] != nil`,
			expected:  "spanevent",
		},
		{
			name:      "spanevent,span,scope,resource",
			statement: `set(span.name, "foo") where spanevent.name != nil and scope.name != nil and resource.attributes["foo"] != nil`,
			expected:  "spanevent",
		},
		{
			name:      "profile,instrumentation_scope,resource",
			statement: `set(profile.name, "foo") where profile.name != nil and instrumentation_scope.name != nil and resource.attributes["foo"] != nil`,
			expected:  "profile",
		},
		{
			name:      "profile,scope,resource",
			statement: `set(profile.name, "foo") where profile.name != nil and scope.name != nil and resource.attributes["foo"] != nil`,
			expected:  "profile",
		},
		{
			name:      "resource",
			statement: `set(resource.attributes["bar"], "foo") where dummy.attributes["foo"] != nil`,
			expected:  "resource",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inferred, err := inferrer.inferFromStatements([]string{tt.statement})
			require.NoError(t, err)
			assert.Equal(t, tt.expected, inferred)
		})
	}
}

func Test_NewPriorityContextInferrer_InferConditions_DefaultContextsOrder(t *testing.T) {
	inferrer := newPriorityContextInferrer(componenttest.NewNopTelemetrySettings(), map[string]*priorityContextInferrerCandidate{
		"log":                   newDummyPriorityContextInferrerCandidate(true, true, []string{"scope", "instrumentation_scope", "resource"}),
		"metric":                newDummyPriorityContextInferrerCandidate(true, true, []string{"datapoint", "scope", "instrumentation_scope", "resource"}),
		"datapoint":             newDummyPriorityContextInferrerCandidate(true, true, []string{"scope", "instrumentation_scope", "resource"}),
		"span":                  newDummyPriorityContextInferrerCandidate(true, true, []string{"spanevent", "scope", "instrumentation_scope", "resource"}),
		"spanevent":             newDummyPriorityContextInferrerCandidate(true, true, []string{"scope", "instrumentation_scope", "resource"}),
		"profile":               newDummyPriorityContextInferrerCandidate(true, true, []string{"profile", "scope", "instrumentation_scope", "resource"}),
		"scope":                 newDummyPriorityContextInferrerCandidate(true, true, []string{"resource"}),
		"instrumentation_scope": newDummyPriorityContextInferrerCandidate(true, true, []string{"resource"}),
		"resource":              newDummyPriorityContextInferrerCandidate(true, true, []string{}),
	})

	tests := []struct {
		name      string
		condition string
		expected  string
	}{
		{
			name:      "log,instrumentation_scope,resource",
			condition: `log.attributes["foo"] !=nil and instrumentation_scope.attributes["foo"] == resource.attributes["foo"]`,
			expected:  "log",
		},
		{
			name:      "log,scope,resource",
			condition: `log.attributes["foo"] != nil and scope.attributes["foo"] == resource.attributes["foo"]`,
			expected:  "log",
		},
		{
			name:      "instrumentation_scope,resource",
			condition: `instrumentation_scope.attributes["foo"] != nil and resource.attributes["foo"] != nil`,
			expected:  "instrumentation_scope",
		},
		{
			name:      "scope,resource",
			condition: `scope.attributes["foo"] != nil and resource.attributes["foo"] != nil`,
			expected:  "scope",
		},
		{
			name:      "metric,instrumentation_scope,resource",
			condition: `metric.name != nil and instrumentation_scope.name != nil and resource.attributes["foo"] != nil`,
			expected:  "metric",
		},
		{
			name:      "metric,scope,resource",
			condition: `metric.name != nil and scope.name != nil and resource.attributes["foo"] != nil`,
			expected:  "metric",
		},
		{
			name:      "datapoint,metric,instrumentation_scope,resource",
			condition: `metric.name != nil and datapoint.double_value > 0 and instrumentation_scope.name != nil and resource.attributes["foo"] != nil`,
			expected:  "datapoint",
		},
		{
			name:      "datapoint,metric,scope,resource",
			condition: `metric.name != nil and datapoint.double_value > 0 and scope.name != nil and resource.attributes["foo"] != nil`,
			expected:  "datapoint",
		},
		{
			name:      "span,instrumentation_scope,resource",
			condition: `span.name != nil and instrumentation_scope.name != nil and resource.attributes["foo"] != nil`,
			expected:  "span",
		},
		{
			name:      "span,scope,resource",
			condition: `span.name != nil and scope.name != nil and resource.attributes["foo"] != nil`,
			expected:  "span",
		},
		{
			name:      "spanevent,span,instrumentation_scope,resource",
			condition: `span.name != nil and spanevent.name != nil and instrumentation_scope.name != nil and resource.attributes["foo"] != nil`,
			expected:  "spanevent",
		},
		{
			name:      "spanevent,span,scope,resource",
			condition: `span.name != nil and spanevent.name != nil and scope.name != nil and resource.attributes["foo"] != nil`,
			expected:  "spanevent",
		},
		{
			name:      "profile,instrumentation_scope,resource",
			condition: `profile.name != nil and profile.name != nil and instrumentation_scope.name != nil and resource.attributes["foo"] != nil`,
			expected:  "profile",
		},
		{
			name:      "profile,scope,resource",
			condition: `profile.name != nil and profile.name != nil and scope.name != nil and resource.attributes["foo"] != nil`,
			expected:  "profile",
		},
		{
			name:      "resource",
			condition: `resource.attributes["bar"] != nil and dummy.attributes["foo"] != nil`,
			expected:  "resource",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inferred, err := inferrer.inferFromConditions([]string{tt.condition})
			require.NoError(t, err)
			assert.Equal(t, tt.expected, inferred)
		})
	}
}

func Test_NewPriorityContextInferrer_Infer(t *testing.T) {
	tests := []struct {
		name       string
		candidates map[string]*priorityContextInferrerCandidate
		statements []string
		conditions []string
		expected   string
	}{
		{
			name: "with statements",
			candidates: map[string]*priorityContextInferrerCandidate{
				"metric":   defaultDummyPriorityContextInferrerCandidate,
				"resource": defaultDummyPriorityContextInferrerCandidate,
			},
			statements: []string{`set(resource.attributes["foo"], "bar")`},
			expected:   "resource",
		},
		{
			name: "with conditions",
			candidates: map[string]*priorityContextInferrerCandidate{
				"metric":   defaultDummyPriorityContextInferrerCandidate,
				"resource": defaultDummyPriorityContextInferrerCandidate,
			},
			conditions: []string{
				`IsMatch(metric.name, "^bar.*")`,
				`IsMatch(metric.name, "^foo.*")`,
			},
			expected: "metric",
		},
		{
			name: "with statements and conditions",
			candidates: map[string]*priorityContextInferrerCandidate{
				"metric":   defaultDummyPriorityContextInferrerCandidate,
				"resource": defaultDummyPriorityContextInferrerCandidate,
			},
			statements: []string{`set(resource.attributes["foo"], "bar")`},
			conditions: []string{
				`IsMatch(metric.name, "^bar.*")`,
				`IsMatch(metric.name, "^foo.*")`,
			},
			expected: "metric",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inferrer := newPriorityContextInferrer(
				componenttest.NewNopTelemetrySettings(),
				tt.candidates,
			)
			inferredContext, err := inferrer.infer(tt.statements, tt.conditions)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, inferredContext)
		})
	}
}
