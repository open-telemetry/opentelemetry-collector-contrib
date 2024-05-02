// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package ptracetest

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/multierr"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/golden"
)

func TestCompareTraces(t *testing.T) {
	tcs := []struct {
		name           string
		compareOptions []CompareTracesOption
		withoutOptions error
		withOptions    error
	}{
		{
			name: "equal",
		},
		{
			name: "ignore-one-resource-attribute",
			compareOptions: []CompareTracesOption{
				IgnoreResourceAttributeValue("host.name"),
			},
			withoutOptions: multierr.Combine(
				errors.New("missing expected resource: map[host.name:different-node1]"),
				errors.New("unexpected resource: map[host.name:host1]"),
			),
			withOptions: nil,
		},
		{
			name: "ignore-resource-order",
			compareOptions: []CompareTracesOption{
				IgnoreResourceSpansOrder(),
			},
			withoutOptions: multierr.Combine(
				errors.New(`resources are out of order: resource "map[host.name:host1]" expected at index 0, found at index 1`),
				errors.New(`resources are out of order: resource "map[host.name:host2]" expected at index 1, found at index 0`),
			),
			withOptions: nil,
		},
		{
			name: "ignore-spanid",
			compareOptions: []CompareTracesOption{
				IgnoreSpanID(),
			},
			withoutOptions: multierr.Combine(
				errors.New("resource \"map[host.name:node1]\": scope \"collector\": span \"span1\": span ID doesn't match expected: fd0da883bb27cd6b, actual: "),
			),
			withOptions: nil,
		},
		{
			name: "ignore-attribute-value",
			compareOptions: []CompareTracesOption{
				IgnoreSpanAttributeValue("testKey2"),
			},
			withoutOptions: multierr.Combine(
				errors.New("resource \"map[host.name:node1]\": scope \"collector\": span \"\": attributes don't match expected: map[testKey2:teststringvalue2], actual: map[testKey2:unpredictable]"),
			),
			withOptions: nil,
		},
		{
			name: "ignore-start-timestamp",
			compareOptions: []CompareTracesOption{
				IgnoreStartTimestamp(),
			},
			withoutOptions: multierr.Combine(
				errors.New("resource \"map[host.name:node1]\": scope \"collector\": span \"span1\": start timestamp doesn't match expected: 11651379494838206464, actual: 0"),
			),
			withOptions: nil,
		},
		{
			name: "ignore-end-timestamp",
			compareOptions: []CompareTracesOption{
				IgnoreEndTimestamp(),
			},
			withoutOptions: multierr.Combine(
				errors.New("resource \"map[host.name:node1]\": scope \"collector\": span \"span1\": end timestamp doesn't match expected: 11651379494838206464, actual: 0"),
			),
			withOptions: nil,
		},
		{
			name: "ignore-traceid",
			compareOptions: []CompareTracesOption{
				IgnoreTraceID(),
			},
			withoutOptions: multierr.Combine(
				errors.New("resource \"map[host.name:node1]\": scope \"collector\": span \"span1\": trace ID doesn't match expected: 8c8b1765a7b0acf0b66aa4623fcb7bd5, actual: "),
			),
			withOptions: nil,
		},
		{
			name: "resourcespans-amount-unequal",
			withoutOptions: multierr.Combine(
				errors.New("number of resources doesn't match expected: 1, actual: 2"),
			),
		},
		{
			name: "resourcespans-attributes-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("missing expected resource: map[pod.name:pod1]"),
				errors.New("unexpected resource: map[pod.name:pod2]"),
			),
		},
		{
			name: "scopespans-amount-unequal",
			withoutOptions: multierr.Combine(
				errors.New("resource \"map[host.name:host1]\": number of scopes doesn't match expected: 1, actual: 2"),
			),
		},
		{
			name: "scopespans-scope-name-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("resource \"map[host.name:host1]\": missing expected scope: scope3; resource \"map[host.name:host1]\": unexpected scope: scope2"),
			),
		},
		{
			name: "scopespans-scope-version-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("resource \"map[host.name:host1]\": scope \"scope2\": version doesn't match expected: v0.2.0, actual: v0.1.0"),
			),
		},
		{
			name: "scopespans-spans-amount-unequal",
			withoutOptions: multierr.Combine(
				errors.New("resource \"map[host.name:node1]\": scope \"collector\": number of spans doesn't match expected: 1, actual: 2"),
			),
		},
		{
			name: "scopespans-spans-attributes-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("resource \"map[host.name:node1]\": scope \"collector\": span \"\": attributes don't match expected: map[key2:value2], actual: map[key2:value3]"),
			),
		},
		{
			name: "scopespans-spans-traceid-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("resource \"map[host.name:node1]\": scope \"collector\": span \"\": trace ID doesn't match expected: 8c8b1765a7b0acf0b66aa4623fcb7bd5, actual: b8cb1765a7b0acf0b66aa4623fcb7bd5"),
			),
		},
		{
			name: "scopespans-spans-spanid-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("resource \"map[host.name:node1]\": scope \"collector\": span \"\": span ID doesn't match expected: fd0da883bb27cd6b, actual: d0dfa883bb27cd6b"),
			),
		},
		{
			name: "scopespans-spans-tracestate-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("resource \"map[host.name:node1]\": scope \"collector\": span \"\": trace state doesn't match expected: xx, actual: yy"),
			),
		},
		{
			name: "scopespans-spans-parentspanid-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("resource \"map[host.name:node1]\": scope \"collector\": span \"\": parent span ID doesn't match expected: bcff497b5a47310f, actual: 310fbcff497b5a47"),
			),
		},
		{
			name: "scopespans-spans-name-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("resource \"map[host.name:node1]\": scope \"collector\": missing expected span: span2"),
				errors.New("resource \"map[host.name:node1]\": scope \"collector\": unexpected span: span1"),
			),
		},
		{
			name: "scopespans-spans-kind-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("resource \"map[host.name:node1]\": scope \"collector\": span \"span1\": kind doesn't match expected: 2, actual: 1"),
			),
		},
		{
			name: "scopespans-spans-starttimestamp-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("resource \"map[host.name:node1]\": scope \"collector\": span \"span1\": start timestamp doesn't match expected: 11651379494838206464, actual: 11651379494838206400"),
			),
		},
		{
			name: "scopespans-spans-endtimestamp-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("resource \"map[host.name:node1]\": scope \"collector\": span \"span1\": end timestamp doesn't match expected: 11651379494838206464, actual: 11651379494838206400"),
			),
		},
		{
			name: "scopespans-spans-droppedattributescount-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("resource \"map[host.name:node1]\": scope \"collector\": span \"span1\": dropped attributes count doesn't match expected: 0, actual: 1"),
			),
		},
		{
			name: "scopespans-spans-events-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("resource \"map[host.name:node1]\": scope \"collector\": span \"\": number of events doesn't match expected: 1, actual: 0"),
			),
		},
		{
			name: "scopespans-spans-events-attributes-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("resource \"map[host.name:node1]\": scope \"collector\": span \"\": span event \"event1\": attributes don't match expected: map[eventkey1:value1], actual: map[eventkey1:value2]"),
			),
		},
		{
			name: "scopespans-spans-events-dropattributescount-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("resource \"map[host.name:node1]\": scope \"collector\": span \"\": span event \"event1\": dropped attributes count doesn't match expected: 1, actual: 0"),
			),
		},
		{
			name: "scopespans-spans-events-name-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("resource \"map[host.name:node1]\": scope \"collector\": span \"\": missing expected span event: event1; resource \"map[host.name:node1]\": scope \"collector\": span \"\": unexpected span event: event2"),
			),
		},
		{
			name: "scopespans-spans-events-timestamp-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("resource \"map[host.name:node1]\": scope \"collector\": span \"\": span event \"event1\": timestamp doesn't match expected: 11651379494838206400, actual: 11651379494838206464"),
			),
		},
		{
			name: "scopespans-spans-droppedeventscount-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("resource \"map[host.name:node1]\": scope \"collector\": span \"span1\": dropped events count doesn't match expected: 0, actual: 1"),
			),
		},
		{
			name: "scopespans-spans-links-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("resource \"map[host.name:node1]\": scope \"collector\": span \"\": number of span links doesn't match expected: 1, actual: 0"),
			),
		},
		{
			name: "scopespans-spans-links-traceid-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("resource \"map[host.name:node1]\": scope \"collector\": span \"\": span link \"\": trace ID doesn't match expected: 8c8b1765a7b0acf0b66aa4623fcb7bd5, actual: "),
			),
		},
		{
			name: "scopespans-spans-links-spanid-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("resource \"map[host.name:node1]\": scope \"collector\": span \"\": missing expected span link: ; resource \"map[host.name:node1]\": scope \"collector\": span \"\": unexpected span link: fd0da883bb27cd6b"),
			),
		},
		{
			name: "scopespans-spans-links-tracestate-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("resource \"map[host.name:node1]\": scope \"collector\": span \"\": span link \"fd0da883bb27cd6b\": trace state doesn't match expected: placeholder, actual: "),
			),
		},
		{
			name: "scopespans-spans-links-attributes-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("resource \"map[host.name:node1]\": scope \"collector\": span \"\": span link \"fd0da883bb27cd6b\": attributes don't match expected: map[testKey1:teststringvalue1], actual: map[testKey1:teststringvalue2]"),
			),
		},
		{
			name: "scopespans-spans-links-dropattributescount-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("resource \"map[host.name:node1]\": scope \"collector\": span \"\": span link \"fd0da883bb27cd6b\": dropped attributes count doesn't match expected: 1, actual: 0"),
			),
		},
		{
			name: "scopespans-spans-droppedlinkscount-mismatch",
			withoutOptions: multierr.Combine(
				errors.New("resource \"map[host.name:node1]\": scope \"collector\": span \"span1\": dropped links count doesn't match expected: 0, actual: 1"),
			),
		},
		{
			name: "scopespans-spans-status-mismatch",
			withoutOptions: multierr.Combine(
				errors.New(`resource "map[host.name:node1]": scope "collector": span "": status code doesn't match expected: Ok, actual: Unset`),
			),
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			dir := filepath.Join("testdata", tc.name)

			expected, err := golden.ReadTraces(filepath.Join(dir, "expected.yaml"))
			require.NoError(t, err)

			actual, err := golden.ReadTraces(filepath.Join(dir, "actual.yaml"))
			require.NoError(t, err)

			err = CompareTraces(expected, actual)
			if tc.withoutOptions == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.withoutOptions.Error())
			}

			if tc.compareOptions == nil {
				return
			}

			err = CompareTraces(expected, actual, tc.compareOptions...)
			if tc.withOptions == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, tc.withOptions, err.Error())
			}
		})
	}
}

func TestCompareResourceSpans(t *testing.T) {
	tests := []struct {
		name     string
		expected ptrace.ResourceSpans
		actual   ptrace.ResourceSpans
		err      error
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
			err: errors.New("attributes don't match expected: map[host.name:host1], actual: map[host.name:host2]"),
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
			err: errors.New("number of scopes doesn't match expected: 2, actual: 1"),
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
			err: multierr.Combine(
				errors.New("missing expected scope: scope1"),
				errors.New("unexpected scope: scope2"),
			),
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
			err: multierr.Combine(
				errors.New("scopes are out of order: scope scope1 expected at index 0, found at index 1"),
				errors.New("scopes are out of order: scope scope2 expected at index 1, found at index 0"),
			),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.err, CompareResourceSpans(test.expected, test.actual))
		})
	}
}

func TestCompareScopeSpans(t *testing.T) {
	tests := []struct {
		name     string
		expected ptrace.ScopeSpans
		actual   ptrace.ScopeSpans
		err      error
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
			err: errors.New("name doesn't match expected: scope1, actual: scope2"),
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
			err: errors.New("number of spans doesn't match expected: 2, actual: 1"),
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
			err: multierr.Combine(
				errors.New(`spans are out of order: span "span1" expected at index 0, found at index 1`),
				errors.New(`spans are out of order: span "span2" expected at index 1, found at index 0`),
			),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.err, CompareScopeSpans(test.expected, test.actual))
		})
	}
}

func TestCompareSpan(t *testing.T) {
	tests := []struct {
		name     string
		expected ptrace.Span
		actual   ptrace.Span
		err      error
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
			err: errors.New("name doesn't match expected: span1, actual: span2"),
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
			err: errors.New("start timestamp doesn't match expected: 123, actual: 456"),
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
			err: errors.New("end timestamp doesn't match expected: 123, actual: 456"),
		},
		{
			name: "attributes-mismatch",
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
			err: errors.New("attributes don't match expected: map[attr1:value1 attr2:value2], actual: map[attr1:value1 attr2:value1]"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.err, CompareSpan(test.expected, test.actual))
		})
	}
}
