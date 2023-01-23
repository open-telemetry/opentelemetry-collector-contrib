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

package ptracetest

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/golden"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/internal"
)

func TestCompareTraces(t *testing.T) {
	tcs := []struct {
		name           string
		compareOptions []CompareTracesOption
		withoutOptions internal.Expectation
		withOptions    internal.Expectation
	}{
		{
			name: "equal",
		},
		{
			name: "ignore-one-resource-attribute",
			compareOptions: []CompareTracesOption{
				IgnoreResourceAttributeValue("host.name"),
			},
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("missing expected resource with attributes: map[host.name:different-node1]"),
					errors.New("extra resource with attributes: map[host.name:host1]"),
				),
				Reason: "An unpredictable resource attribute will cause failures if not ignored.",
			},
			withOptions: internal.Expectation{
				Err:    nil,
				Reason: "The unpredictable resource attribute was ignored on each resource that carried it.",
			},
		},
		{
			name: "ignore-resource-order",
			compareOptions: []CompareTracesOption{
				IgnoreResourceSpansOrder(),
			},
			withoutOptions: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ResourceTraces with attributes map[host.name:host1] expected at index 0, found at index 1"),
					errors.New("ResourceTraces with attributes map[host.name:host2] expected at index 1, found at index 0"),
				),
				Reason: "Resource order mismatch will cause failures if not ignored.",
			},
			withOptions: internal.Expectation{
				Err:    nil,
				Reason: "Ignored resource order mismatch should not cause a failure.",
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join("testdata", tc.name)

			expected, err := golden.ReadTraces(filepath.Join(dir, "expected.json"))
			require.NoError(t, err)

			actual, err := golden.ReadTraces(filepath.Join(dir, "actual.json"))
			require.NoError(t, err)

			err = CompareTraces(expected, actual)
			tc.withoutOptions.Validate(t, err)

			if tc.compareOptions == nil {
				return
			}

			err = CompareTraces(expected, actual, tc.compareOptions...)
			tc.withOptions.Validate(t, err)
		})
	}
}

func TestCompareResourceSpans(t *testing.T) {
	tests := []struct {
		name     string
		expected ptrace.ResourceSpans
		actual   ptrace.ResourceSpans
		err      internal.Expectation
	}{
		{
			name: "equal",
			expected: func() ptrace.ResourceSpans {
				rs := ptrace.NewResourceSpans()
				rs.Resource().Attributes().PutStr("host.name", "host1")
				ss := rs.ScopeSpans().AppendEmpty()
				ss.Scope().SetName("scope1")
				s := ss.Spans().AppendEmpty()
				s.SetName("span1")
				s.SetStartTimestamp(123)
				s.SetEndTimestamp(456)
				return rs
			}(),
			actual: func() ptrace.ResourceSpans {
				rs := ptrace.NewResourceSpans()
				rs.Resource().Attributes().PutStr("host.name", "host1")
				ss := rs.ScopeSpans().AppendEmpty()
				ss.Scope().SetName("scope1")
				s := ss.Spans().AppendEmpty()
				s.SetName("span1")
				s.SetStartTimestamp(123)
				s.SetEndTimestamp(456)
				return rs
			}(),
		},
		{
			name: "resource-attributes-mismatch",
			expected: func() ptrace.ResourceSpans {
				rs := ptrace.NewResourceSpans()
				rs.Resource().Attributes().PutStr("host.name", "host1")
				return rs
			}(),
			actual: func() ptrace.ResourceSpans {
				rs := ptrace.NewResourceSpans()
				rs.Resource().Attributes().PutStr("host.name", "host2")
				return rs
			}(),
			err: internal.Expectation{
				Err:    errors.New("resource attributes do not match expected: map[host.name:host1], actual: map[host.name:host2]"),
				Reason: "Different resources should cause a failure.",
			},
		},
		{
			name: "scopes-number-mismatch",
			expected: func() ptrace.ResourceSpans {
				rs := ptrace.NewResourceSpans()
				rs.ScopeSpans().AppendEmpty()
				rs.ScopeSpans().AppendEmpty()
				return rs
			}(),
			actual: func() ptrace.ResourceSpans {
				rs := ptrace.NewResourceSpans()
				rs.ScopeSpans().AppendEmpty()
				return rs
			}(),
			err: internal.Expectation{
				Err:    errors.New("number of scope spans does not match expected: 2, actual: 1"),
				Reason: "Different number of scope spans should cause a failure.",
			},
		},
		{
			name: "scope-name-mismatch",
			expected: func() ptrace.ResourceSpans {
				rs := ptrace.NewResourceSpans()
				ss := rs.ScopeSpans().AppendEmpty()
				ss.Scope().SetName("scope1")
				return rs
			}(),
			actual: func() ptrace.ResourceSpans {
				rs := ptrace.NewResourceSpans()
				ss := rs.ScopeSpans().AppendEmpty()
				ss.Scope().SetName("scope2")
				return rs
			}(),
			err: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ScopeSpans missing with scope name: scope1"),
					errors.New("unexpected ScopeSpans with scope name: scope2"),
				),
				Reason: "Different scope names should cause a failure.",
			},
		},
		{
			name: "scopes-order-mismatch",
			expected: func() ptrace.ResourceSpans {
				rs := ptrace.NewResourceSpans()
				rs.ScopeSpans().AppendEmpty().Scope().SetName("scope1")
				rs.ScopeSpans().AppendEmpty().Scope().SetName("scope2")
				return rs
			}(),
			actual: func() ptrace.ResourceSpans {
				rs := ptrace.NewResourceSpans()
				rs.ScopeSpans().AppendEmpty().Scope().SetName("scope2")
				rs.ScopeSpans().AppendEmpty().Scope().SetName("scope1")
				return rs
			}(),
			err: internal.Expectation{
				Err: multierr.Combine(
					errors.New("ScopeSpans with scope name scope1 expected at index 0, found at index 1"),
					errors.New("ScopeSpans with scope name scope2 expected at index 1, found at index 0"),
				),
				Reason: "Different scope spans order should cause a failure.",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := CompareResourceSpans(test.expected, test.actual)
			test.err.Validate(t, err)
		})
	}
}

func TestCompareScopeSpans(t *testing.T) {
	tests := []struct {
		name     string
		expected ptrace.ScopeSpans
		actual   ptrace.ScopeSpans
		err      internal.Expectation
	}{
		{
			name: "equal",
			expected: func() ptrace.ScopeSpans {
				ss := ptrace.NewScopeSpans()
				ss.Scope().SetName("scope1")
				s := ss.Spans().AppendEmpty()
				s.SetName("span1")
				s.SetStartTimestamp(123)
				s.SetEndTimestamp(456)
				return ss
			}(),
			actual: func() ptrace.ScopeSpans {
				ss := ptrace.NewScopeSpans()
				ss.Scope().SetName("scope1")
				s := ss.Spans().AppendEmpty()
				s.SetName("span1")
				s.SetStartTimestamp(123)
				s.SetEndTimestamp(456)
				return ss
			}(),
		},
		{
			name: "scope-name-mismatch",
			expected: func() ptrace.ScopeSpans {
				ss := ptrace.NewScopeSpans()
				ss.Scope().SetName("scope1")
				return ss
			}(),
			actual: func() ptrace.ScopeSpans {
				ss := ptrace.NewScopeSpans()
				ss.Scope().SetName("scope2")
				return ss
			}(),
			err: internal.Expectation{
				Err:    errors.New("scope Name does not match expected: scope1, actual: scope2"),
				Reason: "Different scope names should cause a failure.",
			},
		},
		{
			name: "spans-number-mismatch",
			expected: func() ptrace.ScopeSpans {
				ss := ptrace.NewScopeSpans()
				ss.Spans().AppendEmpty()
				ss.Spans().AppendEmpty()
				return ss
			}(),
			actual: func() ptrace.ScopeSpans {
				ss := ptrace.NewScopeSpans()
				ss.Spans().AppendEmpty()
				return ss
			}(),
			err: internal.Expectation{
				Err:    errors.New("number of spans does not match expected: 2, actual: 1"),
				Reason: "Different number of spans should cause a failure.",
			},
		},
		{
			name: "spans-order-mismatch",
			expected: func() ptrace.ScopeSpans {
				ss := ptrace.NewScopeSpans()
				ss.Spans().AppendEmpty().SetName("span1")
				ss.Spans().AppendEmpty().SetName("span2")
				return ss
			}(),
			actual: func() ptrace.ScopeSpans {
				ss := ptrace.NewScopeSpans()
				ss.Spans().AppendEmpty().SetName("span2")
				ss.Spans().AppendEmpty().SetName("span1")
				return ss
			}(),
			err: internal.Expectation{
				Err: multierr.Combine(
					errors.New("span span1 expected at index 0, found at index 1"),
					errors.New("span span2 expected at index 1, found at index 0"),
				),
				Reason: "Different span order should cause a failure.",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := CompareScopeSpans(test.expected, test.actual)
			test.err.Validate(t, err)
		})
	}
}

func TestCompareSpan(t *testing.T) {
	tests := []struct {
		name     string
		expected ptrace.Span
		actual   ptrace.Span
		err      internal.Expectation
	}{
		{
			name: "equal",
			expected: func() ptrace.Span {
				s := ptrace.NewSpan()
				s.SetName("span1")
				s.SetStartTimestamp(123)
				s.SetEndTimestamp(456)
				return s
			}(),
			actual: func() ptrace.Span {
				s := ptrace.NewSpan()
				s.SetName("span1")
				s.SetStartTimestamp(123)
				s.SetEndTimestamp(456)
				return s
			}(),
		},
		{
			name: "name-mismatch",
			expected: func() ptrace.Span {
				s := ptrace.NewSpan()
				s.SetName("span1")
				return s
			}(),
			actual: func() ptrace.Span {
				s := ptrace.NewSpan()
				s.SetName("span2")
				return s
			}(),
			err: internal.Expectation{
				Err:    errors.New("span Name doesn't match expected: span1, actual: span2"),
				Reason: "Different span names should cause a failure.",
			},
		},
		{
			name: "start-timestamp-mismatch",
			expected: func() ptrace.Span {
				s := ptrace.NewSpan()
				s.SetStartTimestamp(123)
				return s
			}(),
			actual: func() ptrace.Span {
				s := ptrace.NewSpan()
				s.SetStartTimestamp(456)
				return s
			}(),
			err: internal.Expectation{
				Err:    errors.New("span StartTimestamp doesn't match expected: 123, actual: 456"),
				Reason: "Different span start timestamps should cause a failure.",
			},
		},
		{
			name: "end-timestamp-mismatch",
			expected: func() ptrace.Span {
				s := ptrace.NewSpan()
				s.SetEndTimestamp(123)
				return s
			}(),
			actual: func() ptrace.Span {
				s := ptrace.NewSpan()
				s.SetEndTimestamp(456)
				return s
			}(),
			err: internal.Expectation{
				Err:    errors.New("span EndTimestamp doesn't match expected: 123, actual: 456"),
				Reason: "Different span end timestamps should cause a failure.",
			},
		},
		{
			name: "attributes-number-mismatch",
			expected: func() ptrace.Span {
				s := ptrace.NewSpan()
				s.Attributes().PutStr("attr1", "value1")
				s.Attributes().PutStr("attr2", "value2")
				return s
			}(),
			actual: func() ptrace.Span {
				s := ptrace.NewSpan()
				s.Attributes().PutStr("attr1", "value1")
				s.Attributes().PutStr("attr2", "value1")
				return s
			}(),
			err: internal.Expectation{
				Err:    errors.New("span attributes do not match expected: map[attr1:value1 attr2:value2], actual: map[attr1:value1 attr2:value1]"),
				Reason: "Different span attributes should cause a failure.",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := CompareSpan(test.expected, test.actual)
			test.err.Validate(t, err)
		})
	}
}
