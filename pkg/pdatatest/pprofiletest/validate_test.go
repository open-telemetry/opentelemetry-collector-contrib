// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofiletest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
)

func Test_validateProfile(t *testing.T) {
	tests := []struct {
		name    string
		profile pprofile.Profile
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name:    "empty string table",
			profile: pprofile.NewProfile(),
			wantErr: assert.Error,
		},
		{
			name: "no empty string at index 0",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.StringTable().Append("x")
				return pp
			}(),
			wantErr: assert.Error,
		},
		{
			name: "empty sample type",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.StringTable().Append("")
				return pp
			}(),
			wantErr: assert.Error,
		},
		{
			name: "invalid sample type",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.StringTable().Append("")
				pp.SampleType().AppendEmpty()
				return pp
			}(),
			wantErr: assert.Error,
		},
		{
			name: "invalid sample",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.StringTable().Append("")
				st := pp.SampleType().AppendEmpty()
				st.SetAggregationTemporality(pprofile.AggregationTemporalityDelta)
				pp.PeriodType().SetAggregationTemporality(pprofile.AggregationTemporalityDelta)
				pp.Sample().AppendEmpty()
				return pp
			}(),
			wantErr: assert.Error,
		},
		{
			name: "invalid default sample type string index",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.StringTable().Append("")
				st := pp.SampleType().AppendEmpty()
				st.SetAggregationTemporality(pprofile.AggregationTemporalityDelta)
				pp.PeriodType().SetAggregationTemporality(pprofile.AggregationTemporalityDelta)
				s := pp.Sample().AppendEmpty()
				s.Value().Append(0)
				pp.SetDefaultSampleTypeStrindex(1)
				return pp
			}(),
			wantErr: assert.Error,
		},
		{
			name: "invalid comment string index",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.StringTable().Append("")
				st := pp.SampleType().AppendEmpty()
				st.SetAggregationTemporality(pprofile.AggregationTemporalityDelta)
				pp.PeriodType().SetAggregationTemporality(pprofile.AggregationTemporalityDelta)
				s := pp.Sample().AppendEmpty()
				s.Value().Append(0)
				pp.SetDefaultSampleTypeStrindex(0)
				pp.CommentStrindices().Append(1)
				return pp
			}(),
			wantErr: assert.Error,
		},
		{
			name: "invalid attribute index",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.StringTable().Append("")
				st := pp.SampleType().AppendEmpty()
				st.SetAggregationTemporality(pprofile.AggregationTemporalityDelta)
				pp.PeriodType().SetAggregationTemporality(pprofile.AggregationTemporalityDelta)
				s := pp.Sample().AppendEmpty()
				s.Value().Append(0)
				pp.SetDefaultSampleTypeStrindex(0)
				pp.CommentStrindices().Append(0)
				pp.AttributeIndices().Append(1)
				return pp
			}(),
			wantErr: assert.Error,
		},
		{
			name: "invalid attribute unit index",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.StringTable().Append("")
				st := pp.SampleType().AppendEmpty()
				st.SetAggregationTemporality(pprofile.AggregationTemporalityDelta)
				pp.PeriodType().SetAggregationTemporality(pprofile.AggregationTemporalityDelta)
				s := pp.Sample().AppendEmpty()
				s.Value().Append(0)
				pp.SetDefaultSampleTypeStrindex(0)
				pp.CommentStrindices().Append(0)
				pp.AttributeIndices().Append(0)
				au := pp.AttributeUnits().AppendEmpty()
				au.SetAttributeKeyStrindex(1)
				return pp
			}(),
			wantErr: assert.Error,
		},
		{
			name: "valid",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.StringTable().Append("")
				st := pp.SampleType().AppendEmpty()
				st.SetAggregationTemporality(pprofile.AggregationTemporalityDelta)
				pp.PeriodType().SetAggregationTemporality(pprofile.AggregationTemporalityDelta)
				s := pp.Sample().AppendEmpty()
				s.Value().Append(0)
				pp.SetDefaultSampleTypeStrindex(0)
				pp.CommentStrindices().Append(0)
				pp.AttributeIndices().Append(0)
				au := pp.AttributeUnits().AppendEmpty()
				au.SetAttributeKeyStrindex(0)
				return pp
			}(),
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantErr(t, ValidateProfile(tt.profile))
		})
	}
}

func Test_validateIndices(t *testing.T) {
	indices := pcommon.NewInt32Slice()
	indices.Append(0, 1)

	assert.Error(t, validateIndices(0, indices))
	assert.Error(t, validateIndices(1, indices))
	assert.NoError(t, validateIndices(2, indices))
	assert.NoError(t, validateIndices(3, indices))
}

func Test_validateIndex(t *testing.T) {
	assert.Error(t, validateIndex(0, 0))
	assert.NoError(t, validateIndex(1, 0))
	assert.Error(t, validateIndex(1, 1))
}

func Test_validateSampleTypes(t *testing.T) {
	tests := []struct {
		name    string
		profile pprofile.Profile
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name:    "empty",
			profile: pprofile.NewProfile(),
			wantErr: assert.NoError,
		},
		{
			name: "valid",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.StringTable().Append("")
				s := pp.SampleType().AppendEmpty()
				s.SetAggregationTemporality(pprofile.AggregationTemporalityDelta)
				s = pp.SampleType().AppendEmpty()
				s.SetAggregationTemporality(pprofile.AggregationTemporalityCumulative)
				return pp
			}(),
			wantErr: assert.NoError,
		},
		{
			name: "invalid",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.StringTable().Append("")
				s := pp.SampleType().AppendEmpty()
				s.SetAggregationTemporality(pprofile.AggregationTemporalityDelta)
				s = pp.SampleType().AppendEmpty()
				s.SetAggregationTemporality(3)
				return pp
			}(),
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantErr(t, validateSampleType(tt.profile))
		})
	}
}

func Test_validateValueType(t *testing.T) {
	tests := []struct {
		name      string
		profile   pprofile.Profile
		valueType pprofile.ValueType
		wantErr   assert.ErrorAssertionFunc
	}{
		{
			name:      "type string index out of range",
			profile:   pprofile.NewProfile(),
			valueType: pprofile.NewValueType(),
			wantErr:   assert.Error,
		},
		{
			name: "invalid aggregation temporality",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.StringTable().Append("")
				return pp
			}(),
			valueType: pprofile.NewValueType(),
			wantErr:   assert.Error,
		},
		{
			name: "unit string index out of range",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.StringTable().Append("")
				return pp
			}(),
			valueType: func() pprofile.ValueType {
				pp := pprofile.NewValueType()
				pp.SetAggregationTemporality(pprofile.AggregationTemporalityDelta)
				pp.SetUnitStrindex(1)
				return pp
			}(),
			wantErr: assert.Error,
		},
		{
			name: "valid delta",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.StringTable().Append("")
				return pp
			}(),
			valueType: func() pprofile.ValueType {
				pp := pprofile.NewValueType()
				pp.SetAggregationTemporality(pprofile.AggregationTemporalityDelta)
				return pp
			}(),
			wantErr: assert.NoError,
		},
		{
			name: "valid cumulative",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.StringTable().Append("")
				return pp
			}(),
			valueType: func() pprofile.ValueType {
				pp := pprofile.NewValueType()
				pp.SetAggregationTemporality(pprofile.AggregationTemporalityCumulative)
				return pp
			}(),
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantErr(t, validateValueType(tt.profile.StringTable().Len(), tt.valueType))
		})
	}
}

func Test_validateSamples(t *testing.T) {
	tests := []struct {
		name    string
		profile pprofile.Profile
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name:    "no samples",
			profile: pprofile.NewProfile(),
			wantErr: assert.NoError,
		},
		{
			name: "valid samples",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.Sample().AppendEmpty()
				pp.Sample().AppendEmpty()
				return pp
			}(),
			wantErr: assert.NoError,
		},
		{
			name: "invalid sample",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.Sample().AppendEmpty()
				s := pp.Sample().AppendEmpty()
				s.TimestampsUnixNano().Append(123)
				return pp
			}(),
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantErr(t, validateSamples(tt.profile))
		})
	}
}

func Test_validateSample(t *testing.T) {
	tests := []struct {
		name    string
		profile pprofile.Profile
		sample  pprofile.Sample
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name:    "empty",
			profile: pprofile.NewProfile(),
			sample:  pprofile.NewSample(),
			wantErr: assert.NoError,
		},
		{
			name:    "negative location length",
			profile: pprofile.NewProfile(),
			sample: func() pprofile.Sample {
				s := pprofile.NewSample()
				s.SetLocationsLength(-1)
				return s
			}(),
			wantErr: assert.Error,
		},
		{
			name:    "location length out of range",
			profile: pprofile.NewProfile(),
			sample: func() pprofile.Sample {
				s := pprofile.NewSample()
				s.SetLocationsStartIndex(0)
				s.SetLocationsLength(1)
				return s
			}(),
			wantErr: assert.Error,
		},
		{
			name: "location start plus location length in range",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.LocationTable().AppendEmpty()
				pp.LocationIndices().Append(0)
				return pp
			}(),
			sample: func() pprofile.Sample {
				s := pprofile.NewSample()
				s.SetLocationsStartIndex(0)
				s.SetLocationsLength(1)
				return s
			}(),
			wantErr: assert.NoError,
		},
		{
			name: "location start plus location length out of range",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.LocationTable().AppendEmpty()
				pp.LocationIndices().Append(0)
				return pp
			}(),
			sample: func() pprofile.Sample {
				s := pprofile.NewSample()
				s.SetLocationsStartIndex(0)
				s.SetLocationsLength(2)
				return s
			}(),
			wantErr: assert.Error,
		},
		{
			name: "location index out of range",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.LocationTable().AppendEmpty()
				pp.LocationIndices().Append(1)
				return pp
			}(),
			sample: func() pprofile.Sample {
				s := pprofile.NewSample()
				s.SetLocationsStartIndex(0)
				s.SetLocationsLength(1)
				return s
			}(),
			wantErr: assert.Error,
		},
		{
			name:    "sample type length does not match",
			profile: pprofile.NewProfile(),
			sample: func() pprofile.Sample {
				s := pprofile.NewSample()
				s.Value().Append(123)
				return s
			}(),
			wantErr: assert.Error,
		},
		{
			name: "attribute in range",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.AttributeTable().AppendEmpty()
				return pp
			}(),
			sample: func() pprofile.Sample {
				s := pprofile.NewSample()
				s.AttributeIndices().Append(0)
				return s
			}(),
			wantErr: assert.NoError,
		},
		{
			name: "attribute out of range",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.AttributeTable().AppendEmpty()
				return pp
			}(),
			sample: func() pprofile.Sample {
				s := pprofile.NewSample()
				s.AttributeIndices().Append(1)
				return s
			}(),
			wantErr: assert.Error,
		},
		{
			name: "timestamp in range",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.SetTime(1)
				pp.SetDuration(1)
				return pp
			}(),
			sample: func() pprofile.Sample {
				s := pprofile.NewSample()
				s.TimestampsUnixNano().Append(1)
				return s
			}(),
			wantErr: assert.NoError,
		},
		{
			name: "timestamp too small",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.SetTime(1)
				pp.SetDuration(1)
				return pp
			}(),
			sample: func() pprofile.Sample {
				s := pprofile.NewSample()
				s.TimestampsUnixNano().Append(0)
				return s
			}(),
			wantErr: assert.Error,
		},
		{
			name: "timestamp too high",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.SetTime(1)
				pp.SetDuration(1)
				return pp
			}(),
			sample: func() pprofile.Sample {
				s := pprofile.NewSample()
				s.TimestampsUnixNano().Append(2)
				return s
			}(),
			wantErr: assert.Error,
		},
		{
			name: "link in range",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.LinkTable().AppendEmpty()
				return pp
			}(),
			sample: func() pprofile.Sample {
				s := pprofile.NewSample()
				s.SetLinkIndex(0)
				return s
			}(),
			wantErr: assert.NoError,
		},
		{
			name: "link out of range",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.LinkTable().AppendEmpty()
				return pp
			}(),
			sample: func() pprofile.Sample {
				s := pprofile.NewSample()
				s.SetLinkIndex(1)
				return s
			}(),
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantErr(t, validateSample(tt.profile, tt.sample))
		})
	}
}

func Test_validateLocation(t *testing.T) {
	tests := []struct {
		name     string
		profile  pprofile.Profile
		location pprofile.Location
		wantErr  assert.ErrorAssertionFunc
	}{
		{
			name:     "empty",
			profile:  pprofile.NewProfile(),
			location: pprofile.NewLocation(),
			wantErr:  assert.NoError,
		},
		{
			name: "mapping index in range",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.StringTable().Append("")
				pp.MappingTable().AppendEmpty()
				return pp
			}(),
			location: func() pprofile.Location {
				l := pprofile.NewLocation()
				l.SetMappingIndex(0)
				return l
			}(),
			wantErr: assert.NoError,
		},
		{
			name: "with line and attribute",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.StringTable().Append("")
				pp.MappingTable().AppendEmpty()
				pp.AttributeTable().AppendEmpty()
				pp.FunctionTable().AppendEmpty()
				return pp
			}(),
			location: func() pprofile.Location {
				l := pprofile.NewLocation()
				l.AttributeIndices().Append(0)
				l.Line().AppendEmpty()
				return l
			}(),
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantErr(t, validateLocation(tt.profile, tt.location))
		})
	}
}

func Test_validateLine(t *testing.T) {
	tests := []struct {
		name    string
		profile pprofile.Profile
		line    pprofile.Line
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name:    "function index out of range",
			profile: pprofile.NewProfile(),
			line:    pprofile.NewLine(),
			wantErr: assert.Error,
		},
		{
			name: "function index in range",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.StringTable().Append("")
				pp.FunctionTable().AppendEmpty()
				return pp
			}(),
			line:    pprofile.NewLine(),
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantErr(t, validateLine(tt.profile, tt.line))
		})
	}
}

func Test_validateMapping(t *testing.T) {
	tests := []struct {
		name    string
		profile pprofile.Profile
		mapping pprofile.Mapping
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name:    "filename index out of range",
			profile: pprofile.NewProfile(),
			mapping: pprofile.NewMapping(),
			wantErr: assert.Error,
		},
		{
			name: "filename index in range",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.StringTable().Append("")
				return pp
			}(),
			mapping: pprofile.NewMapping(),
			wantErr: assert.NoError,
		},
		{
			name: "attribute out of range",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.StringTable().Append("")
				return pp
			}(),
			mapping: func() pprofile.Mapping {
				m := pprofile.NewMapping()
				m.AttributeIndices().Append(0)
				return m
			}(),
			wantErr: assert.Error,
		},
		{
			name: "attribute in range",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.StringTable().Append("")
				pp.AttributeTable().AppendEmpty()
				return pp
			}(),
			mapping: func() pprofile.Mapping {
				m := pprofile.NewMapping()
				m.AttributeIndices().Append(0)
				return m
			}(),
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantErr(t, validateMapping(tt.profile, tt.mapping))
		})
	}
}

func Test_validateAttributeUnits(t *testing.T) {
	tests := []struct {
		name    string
		profile pprofile.Profile
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name:    "empty",
			profile: pprofile.NewProfile(),
			wantErr: assert.NoError,
		},
		{
			name: "in range",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.StringTable().Append("")
				pp.AttributeUnits().AppendEmpty()
				return pp
			}(),
			wantErr: assert.NoError,
		},
		{
			name: "unit index out of range",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.StringTable().Append("")
				au := pp.AttributeUnits().AppendEmpty()
				au.SetUnitStrindex(1)
				return pp
			}(),
			wantErr: assert.Error,
		},
		{
			name: "attribute key index out of range",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.StringTable().Append("")
				au := pp.AttributeUnits().AppendEmpty()
				au.SetAttributeKeyStrindex(1)
				return pp
			}(),
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantErr(t, validateAttributeUnits(tt.profile))
		})
	}
}

func Test_validateAttributeUnitAt(t *testing.T) {
	tests := []struct {
		name     string
		profile  pprofile.Profile
		attrUnit pprofile.AttributeUnit
		wantErr  assert.ErrorAssertionFunc
	}{
		{
			name:     "out of range",
			profile:  pprofile.NewProfile(),
			attrUnit: pprofile.NewAttributeUnit(),
			wantErr:  assert.Error,
		},
		{
			name: "in range",
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.StringTable().Append("")
				return pp
			}(),
			attrUnit: pprofile.NewAttributeUnit(),
			wantErr:  assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantErr(t, validateAttributeUnit(tt.profile, tt.attrUnit))
		})
	}
}
