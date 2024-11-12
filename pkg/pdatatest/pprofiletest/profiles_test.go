// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofiletest

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.uber.org/multierr"
)

func TestCompareProfiles(t *testing.T) {
	tcs := []struct {
		name           string
		expected       pprofile.Profiles
		actual         pprofile.Profiles
		compareOptions []CompareProfilesOption
		withoutOptions error
		withOptions    error
	}{}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := CompareProfiles(tc.expected, tc.actual)
			if tc.withoutOptions == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, tc.withoutOptions, err.Error())
			}

			if tc.compareOptions == nil {
				return
			}

			err = CompareProfiles(tc.expected, tc.actual, tc.compareOptions...)
			if tc.withOptions == nil {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, tc.withOptions.Error())
			}
		})
	}
}

func TestCompareResourceProfiles(t *testing.T) {
	tests := []struct {
		name     string
		expected pprofile.ResourceProfiles
		actual   pprofile.ResourceProfiles
		err      error
	}{
		{
			name: "equal",
			expected: func() pprofile.ResourceProfiles {
				rl := pprofile.NewResourceProfiles()
				rl.Resource().Attributes().PutStr("key1", "value1")
				l := rl.ScopeProfiles().AppendEmpty().Profiles().AppendEmpty()
				l.Attributes().PutStr("profile-attr1", "value1")
				return rl
			}(),
			actual: func() pprofile.ResourceProfiles {
				rl := pprofile.NewResourceProfiles()
				rl.Resource().Attributes().PutStr("key1", "value1")
				l := rl.ScopeProfiles().AppendEmpty().Profiles().AppendEmpty()
				l.Attributes().PutStr("profile-attr1", "value1")
				return rl
			}(),
		},
		{
			name: "resource-attributes-mismatch",
			expected: func() pprofile.ResourceProfiles {
				rl := pprofile.NewResourceProfiles()
				rl.Resource().Attributes().PutStr("key1", "value1")
				rl.Resource().Attributes().PutStr("key2", "value2")
				return rl
			}(),
			actual: func() pprofile.ResourceProfiles {
				rl := pprofile.NewResourceProfiles()
				rl.Resource().Attributes().PutStr("key1", "value1")
				return rl
			}(),
			err: errors.New("attributes don't match expected: map[key1:value1 key2:value2], actual: map[key1:value1]"),
		},
		{
			name: "resource-schema-url-mismatch",
			expected: func() pprofile.ResourceProfiles {
				rl := pprofile.NewResourceProfiles()
				rl.SetSchemaUrl("schema-url")
				return rl
			}(),
			actual: func() pprofile.ResourceProfiles {
				rl := pprofile.NewResourceProfiles()
				rl.SetSchemaUrl("schema-url-2")
				return rl
			}(),
			err: errors.New("schema url doesn't match expected: schema-url, actual: schema-url-2"),
		},
		{
			name: "scope-profiles-number-mismatch",
			expected: func() pprofile.ResourceProfiles {
				rl := pprofile.NewResourceProfiles()
				rl.ScopeProfiles().AppendEmpty()
				rl.ScopeProfiles().AppendEmpty()
				return rl
			}(),
			actual: func() pprofile.ResourceProfiles {
				rl := pprofile.NewResourceProfiles()
				rl.ScopeProfiles().AppendEmpty()
				return rl
			}(),
			err: errors.New("number of scopes doesn't match expected: 2, actual: 1"),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.err, CompareResourceProfiles(test.expected, test.actual))
		})
	}
}

func TestCompareScopeProfiles(t *testing.T) {
	tests := []struct {
		name     string
		expected pprofile.ScopeProfiles
		actual   pprofile.ScopeProfiles
		err      error
	}{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.err, CompareScopeProfiles(test.expected, test.actual))
		})
	}
}

func TestCompareProfileContainer(t *testing.T) {
	tests := []struct {
		name     string
		expected pprofile.ProfileContainer
		actual   pprofile.ProfileContainer
		err      error
	}{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.err, CompareProfileContainer(test.expected, test.actual))
		})
	}
}

func TestCompareProfile(t *testing.T) {
	tests := []struct {
		name     string
		expected pprofile.Profile
		actual   pprofile.Profile
		err      error
	}{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.err, CompareProfile(test.expected, test.actual))
		})
	}
}

func TestCompareProfileValueTypeSlice(t *testing.T) {
	tests := []struct {
		name     string
		expected pprofile.ValueTypeSlice
		actual   pprofile.ValueTypeSlice
		err      error
	}{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.err, CompareProfileValueTypeSlice(test.expected, test.actual))
		})
	}
}

func TestCompareProfileSampleSlice(t *testing.T) {
	tests := []struct {
		name     string
		expected pprofile.SampleSlice
		actual   pprofile.SampleSlice
		err      error
	}{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.err, CompareProfileSampleSlice(test.expected, test.actual))
		})
	}
}

func TestCompareProfileSample(t *testing.T) {
	tests := []struct {
		name     string
		expected pprofile.Sample
		actual   pprofile.Sample
		err      error
	}{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.err, CompareProfileSample(test.expected, test.actual))
		})
	}
}

func TestCompareProfileLabelSlice(t *testing.T) {
	tests := []struct {
		name     string
		expected pprofile.LabelSlice
		actual   pprofile.LabelSlice
		err      error
	}{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.err, CompareProfileLabelSlice(test.expected, test.actual))
		})
	}
}

func TestCompareProfileMappingSlice(t *testing.T) {
	tests := []struct {
		name     string
		expected pprofile.MappingSlice
		actual   pprofile.MappingSlice
		err      error
	}{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.err, CompareProfileMappingSlice(test.expected, test.actual))
		})
	}
}

func TestCompareProfileFunctionSlice(t *testing.T) {
	tests := []struct {
		name     string
		expected pprofile.FunctionSlice
		actual   pprofile.FunctionSlice
		err      error
	}{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.err, CompareProfileFunctionSlice(test.expected, test.actual))
		})
	}
}

func TestCompareProfileLocationSlice(t *testing.T) {
	tests := []struct {
		name     string
		expected pprofile.LocationSlice
		actual   pprofile.LocationSlice
		err      error
	}{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.err, CompareProfileLocationSlice(test.expected, test.actual))
		})
	}
}

func TestCompareProfileLocation(t *testing.T) {
	tests := []struct {
		name     string
		expected pprofile.Location
		actual   pprofile.Location
		err      error
	}{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.err, CompareProfileLocation(test.expected, test.actual))
		})
	}
}

func TestCompareProfileLineSlice(t *testing.T) {
	tests := []struct {
		name     string
		expected pprofile.LineSlice
		actual   pprofile.LineSlice
		err      error
	}{

		{
			name: "empty",
			expected: func() pprofile.LineSlice {
				l := pprofile.NewLineSlice()
				return l
			}(),
			actual: func() pprofile.LineSlice {
				l := pprofile.NewLineSlice()
				return l
			}(),
		},
		{
			name: "equal",
			expected: func() pprofile.LineSlice {
				l := pprofile.NewLineSlice()
				i1 := l.AppendEmpty()
				i1.SetFunctionIndex(1)
				i1.SetLine(3)
				i1.SetColumn(3)
				i2 := l.AppendEmpty()
				i2.SetFunctionIndex(2)
				i2.SetLine(4)
				i2.SetColumn(4)
				return l
			}(),
			actual: func() pprofile.LineSlice {
				l := pprofile.NewLineSlice()
				i1 := l.AppendEmpty()
				i1.SetFunctionIndex(1)
				i1.SetLine(3)
				i1.SetColumn(3)
				i2 := l.AppendEmpty()
				i2.SetFunctionIndex(2)
				i2.SetLine(4)
				i2.SetColumn(4)
				return l
			}(),
		},
		{
			name: "equal wrong order",
			expected: func() pprofile.LineSlice {
				l := pprofile.NewLineSlice()
				i1 := l.AppendEmpty()
				i1.SetFunctionIndex(1)
				i1.SetLine(3)
				i1.SetColumn(3)
				i2 := l.AppendEmpty()
				i2.SetFunctionIndex(2)
				i2.SetLine(4)
				i2.SetColumn(4)
				return l
			}(),
			actual: func() pprofile.LineSlice {
				l := pprofile.NewLineSlice()
				i2 := l.AppendEmpty()
				i2.SetFunctionIndex(2)
				i2.SetLine(4)
				i2.SetColumn(4)
				i1 := l.AppendEmpty()
				i1.SetFunctionIndex(1)
				i1.SetLine(3)
				i1.SetColumn(3)
				return l
			}(),
			err: multierr.Combine(
				errors.New(`lines are out of order: line "functionIndex: 1" expected at index 0, found at index 1`),
				errors.New(`lines are out of order: line "functionIndex: 2" expected at index 1, found at index 0`),
			),
		},
		{
			name: "wrong length",
			expected: func() pprofile.LineSlice {
				l := pprofile.NewLineSlice()
				i1 := l.AppendEmpty()
				i1.SetFunctionIndex(1)
				i1.SetLine(3)
				i1.SetColumn(3)
				return l
			}(),
			actual: func() pprofile.LineSlice {
				l := pprofile.NewLineSlice()
				i1 := l.AppendEmpty()
				i1.SetFunctionIndex(1)
				i1.SetLine(3)
				i1.SetColumn(3)
				i2 := l.AppendEmpty()
				i2.SetFunctionIndex(2)
				i2.SetLine(4)
				i2.SetColumn(4)
				return l
			}(),
			err: multierr.Combine(
				errors.New(`number of lines doesn't match expected: 1, actual: 2`),
			),
		},
		{
			name: "not equal - does not match expected",
			expected: func() pprofile.LineSlice {
				l := pprofile.NewLineSlice()
				i1 := l.AppendEmpty()
				i1.SetFunctionIndex(1)
				i1.SetLine(3)
				i1.SetColumn(3)
				i2 := l.AppendEmpty()
				i2.SetFunctionIndex(2)
				i2.SetLine(4)
				i2.SetColumn(4)
				return l
			}(),
			actual: func() pprofile.LineSlice {
				l := pprofile.NewLineSlice()
				i1 := l.AppendEmpty()
				i1.SetFunctionIndex(1)
				i1.SetLine(3)
				i1.SetColumn(3)
				i2 := l.AppendEmpty()
				i2.SetFunctionIndex(2)
				i2.SetLine(5)
				i2.SetColumn(5)
				return l
			}(),
			err: multierr.Combine(
				errors.New(`line with "functionIndex: 2" does not match expected`),
			),
		},
		{
			name: "not equal - missing",
			expected: func() pprofile.LineSlice {
				l := pprofile.NewLineSlice()
				i1 := l.AppendEmpty()
				i1.SetFunctionIndex(1)
				i1.SetLine(3)
				i1.SetColumn(3)
				i2 := l.AppendEmpty()
				i2.SetFunctionIndex(2)
				i2.SetLine(4)
				i2.SetColumn(4)
				return l
			}(),
			actual: func() pprofile.LineSlice {
				l := pprofile.NewLineSlice()
				i1 := l.AppendEmpty()
				i1.SetFunctionIndex(1)
				i1.SetLine(3)
				i1.SetColumn(3)
				i2 := l.AppendEmpty()
				i2.SetFunctionIndex(3)
				i2.SetLine(5)
				i2.SetColumn(5)
				return l
			}(),
			err: multierr.Combine(
				errors.New(`missing expected line "functionIndex: 2"`),
				errors.New(`unexpected profile line "functionIndex: 3"`),
			),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.err, CompareProfileLineSlice(test.expected, test.actual))
		})
	}
}

func TestCompareProfileAttributeUnitSlice(t *testing.T) {
	tests := []struct {
		name     string
		expected pprofile.AttributeUnitSlice
		actual   pprofile.AttributeUnitSlice
		err      error
	}{
		{
			name: "empty",
			expected: func() pprofile.AttributeUnitSlice {
				l := pprofile.NewAttributeUnitSlice()
				return l
			}(),
			actual: func() pprofile.AttributeUnitSlice {
				l := pprofile.NewAttributeUnitSlice()
				return l
			}(),
		},
		{
			name: "equal",
			expected: func() pprofile.AttributeUnitSlice {
				l := pprofile.NewAttributeUnitSlice()
				i1 := l.AppendEmpty()
				i1.SetAttributeKey(2)
				i1.SetUnit(3)
				i2 := l.AppendEmpty()
				i2.SetAttributeKey(4)
				i2.SetUnit(5)
				return l
			}(),
			actual: func() pprofile.AttributeUnitSlice {
				l := pprofile.NewAttributeUnitSlice()
				i1 := l.AppendEmpty()
				i1.SetAttributeKey(2)
				i1.SetUnit(3)
				i2 := l.AppendEmpty()
				i2.SetAttributeKey(4)
				i2.SetUnit(5)
				return l
			}(),
		},
		{
			name: "equal wrong order",
			expected: func() pprofile.AttributeUnitSlice {
				l := pprofile.NewAttributeUnitSlice()
				i1 := l.AppendEmpty()
				i1.SetAttributeKey(2)
				i1.SetUnit(3)
				i2 := l.AppendEmpty()
				i2.SetAttributeKey(4)
				i2.SetUnit(5)
				return l
			}(),
			actual: func() pprofile.AttributeUnitSlice {
				l := pprofile.NewAttributeUnitSlice()
				i2 := l.AppendEmpty()
				i2.SetAttributeKey(4)
				i2.SetUnit(5)
				i1 := l.AppendEmpty()
				i1.SetAttributeKey(2)
				i1.SetUnit(3)
				return l
			}(),
			err: multierr.Combine(
				errors.New(`attributeUnits are out of order: attributeUnit "attributeKey: 2" expected at index 0, found at index 1`),
				errors.New(`attributeUnits are out of order: attributeUnit "attributeKey: 4" expected at index 1, found at index 0`),
			),
		},
		{
			name: "wrong length",
			expected: func() pprofile.AttributeUnitSlice {
				l := pprofile.NewAttributeUnitSlice()
				i1 := l.AppendEmpty()
				i1.SetAttributeKey(2)
				i1.SetUnit(3)
				return l
			}(),
			actual: func() pprofile.AttributeUnitSlice {
				l := pprofile.NewAttributeUnitSlice()
				i1 := l.AppendEmpty()
				i1.SetAttributeKey(2)
				i1.SetUnit(3)
				i2 := l.AppendEmpty()
				i2.SetAttributeKey(4)
				i2.SetUnit(5)
				return l
			}(),
			err: multierr.Combine(
				errors.New(`number of attributeUnits doesn't match expected: 1, actual: 2`),
			),
		},
		{
			name: "not equal",
			expected: func() pprofile.AttributeUnitSlice {
				l := pprofile.NewAttributeUnitSlice()
				i1 := l.AppendEmpty()
				i1.SetAttributeKey(2)
				i1.SetUnit(3)
				i2 := l.AppendEmpty()
				i2.SetAttributeKey(4)
				i2.SetUnit(5)
				return l
			}(),
			actual: func() pprofile.AttributeUnitSlice {
				l := pprofile.NewAttributeUnitSlice()
				i1 := l.AppendEmpty()
				i1.SetAttributeKey(2)
				i1.SetUnit(3)
				i2 := l.AppendEmpty()
				i2.SetAttributeKey(6)
				i2.SetUnit(7)
				return l
			}(),
			err: multierr.Combine(
				errors.New(`missing expected attributeUnit "attributeKey: 4"`),
				errors.New(`unexpected profile attributeUnit "attributeKey: 6"`),
			),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.err, CompareProfileAttributeUnitSlice(test.expected, test.actual))
		})
	}
}

func TestCompareProfileLinkSlice(t *testing.T) {
	tests := []struct {
		name     string
		expected pprofile.LinkSlice
		actual   pprofile.LinkSlice
		err      error
	}{
		{
			name: "empty",
			expected: func() pprofile.LinkSlice {
				l := pprofile.NewLinkSlice()
				return l
			}(),
			actual: func() pprofile.LinkSlice {
				l := pprofile.NewLinkSlice()
				return l
			}(),
		},
		{
			name: "equal",
			expected: func() pprofile.LinkSlice {
				l := pprofile.NewLinkSlice()
				i1 := l.AppendEmpty()
				i1.SetSpanID(pcommon.SpanID([]byte("spanidnn")))
				i1.SetTraceID(pcommon.TraceID([]byte("traceidnnnnnnnnn")))
				i2 := l.AppendEmpty()
				i2.SetSpanID(pcommon.SpanID([]byte("spanidn2")))
				i2.SetTraceID(pcommon.TraceID([]byte("traceid2nnnnnnnn")))
				return l
			}(),
			actual: func() pprofile.LinkSlice {
				l := pprofile.NewLinkSlice()
				i1 := l.AppendEmpty()
				i1.SetSpanID(pcommon.SpanID([]byte("spanidnn")))
				i1.SetTraceID(pcommon.TraceID([]byte("traceidnnnnnnnnn")))
				i2 := l.AppendEmpty()
				i2.SetSpanID(pcommon.SpanID([]byte("spanidn2")))
				i2.SetTraceID(pcommon.TraceID([]byte("traceid2nnnnnnnn")))
				return l
			}(),
		},
		{
			name: "equal wrong order",
			expected: func() pprofile.LinkSlice {
				l := pprofile.NewLinkSlice()
				i1 := l.AppendEmpty()
				i1.SetSpanID(pcommon.SpanID([]byte("spanidnn")))
				i1.SetTraceID(pcommon.TraceID([]byte("traceidnnnnnnnnn")))
				i2 := l.AppendEmpty()
				i2.SetSpanID(pcommon.SpanID([]byte("spanidn2")))
				i2.SetTraceID(pcommon.TraceID([]byte("traceid2nnnnnnnn")))
				return l
			}(),
			actual: func() pprofile.LinkSlice {
				l := pprofile.NewLinkSlice()
				i2 := l.AppendEmpty()
				i2.SetSpanID(pcommon.SpanID([]byte("spanidn2")))
				i2.SetTraceID(pcommon.TraceID([]byte("traceid2nnnnnnnn")))
				i1 := l.AppendEmpty()
				i1.SetSpanID(pcommon.SpanID([]byte("spanidnn")))
				i1.SetTraceID(pcommon.TraceID([]byte("traceidnnnnnnnnn")))
				return l
			}(),
			err: multierr.Combine(
				errors.New(`links are out of order: link "spanId: 7370616e69646e6e, traceId: 747261636569646e6e6e6e6e6e6e6e6e" expected at index 0, found at index 1`),
				errors.New(`links are out of order: link "spanId: 7370616e69646e32, traceId: 74726163656964326e6e6e6e6e6e6e6e" expected at index 1, found at index 0`),
			),
		},
		{
			name: "wrong length",
			expected: func() pprofile.LinkSlice {
				l := pprofile.NewLinkSlice()
				i1 := l.AppendEmpty()
				i1.SetSpanID(pcommon.SpanID([]byte("spanidnn")))
				i1.SetTraceID(pcommon.TraceID([]byte("traceidnnnnnnnnn")))
				return l
			}(),
			actual: func() pprofile.LinkSlice {
				l := pprofile.NewLinkSlice()
				i2 := l.AppendEmpty()
				i2.SetSpanID(pcommon.SpanID([]byte("spanidn2")))
				i2.SetTraceID(pcommon.TraceID([]byte("traceid2nnnnnnnn")))
				i1 := l.AppendEmpty()
				i1.SetSpanID(pcommon.SpanID([]byte("spanidnn")))
				i1.SetTraceID(pcommon.TraceID([]byte("traceidnnnnnnnnn")))
				return l
			}(),
			err: multierr.Combine(
				errors.New(`number of links doesn't match expected: 1, actual: 2`),
			),
		},
		{
			name: "not equal",
			expected: func() pprofile.LinkSlice {
				l := pprofile.NewLinkSlice()
				i1 := l.AppendEmpty()
				i1.SetSpanID(pcommon.SpanID([]byte("spanidnn")))
				i1.SetTraceID(pcommon.TraceID([]byte("traceidnnnnnnnnn")))
				i2 := l.AppendEmpty()
				i2.SetSpanID(pcommon.SpanID([]byte("spanidn3")))
				i2.SetTraceID(pcommon.TraceID([]byte("traceid3nnnnnnnn")))
				return l
			}(),
			actual: func() pprofile.LinkSlice {
				l := pprofile.NewLinkSlice()
				i2 := l.AppendEmpty()
				i2.SetSpanID(pcommon.SpanID([]byte("spanidn2")))
				i2.SetTraceID(pcommon.TraceID([]byte("traceid2nnnnnnnn")))
				i1 := l.AppendEmpty()
				i1.SetSpanID(pcommon.SpanID([]byte("spanidnn")))
				i1.SetTraceID(pcommon.TraceID([]byte("traceidnnnnnnnnn")))
				return l
			}(),
			err: multierr.Combine(
				errors.New(`missing expected link "spanId: 7370616e69646e33, traceId: 74726163656964336e6e6e6e6e6e6e6e"`),
				errors.New(`unexpected profile link "spanId: 7370616e69646e32, traceId: 74726163656964326e6e6e6e6e6e6e6e"`),
			),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.err, CompareProfileLinkSlice(test.expected, test.actual))
		})
	}
}
