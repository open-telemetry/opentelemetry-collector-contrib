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
		name       string
		dictionary pprofile.ProfilesDictionary
		profile    pprofile.Profile
		wantErr    assert.ErrorAssertionFunc
	}{
		{
			name:       "empty string table",
			dictionary: pprofile.NewProfilesDictionary(),
			profile:    pprofile.NewProfile(),
			wantErr:    assert.Error,
		},
		{
			name: "no empty string at index 0",
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append("x")
				return dic
			}(),
			profile: pprofile.NewProfile(),
			wantErr: assert.Error,
		},
		{
			name: "empty sample type",
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append("")
				return dic
			}(),
			profile: pprofile.NewProfile(),
			wantErr: assert.Error,
		},
		{
			name: "invalid sample type",
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append("")
				return dic
			}(),
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.SampleType().AppendEmpty()
				return pp
			}(),
			wantErr: assert.Error,
		},
		{
			name: "invalid sample",
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append("")
				return dic
			}(),
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
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
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append("")
				return dic
			}(),
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				st := pp.SampleType().AppendEmpty()
				st.SetAggregationTemporality(pprofile.AggregationTemporalityDelta)
				pp.PeriodType().SetAggregationTemporality(pprofile.AggregationTemporalityDelta)
				s := pp.Sample().AppendEmpty()
				s.Value().Append(0)
				pp.SetDefaultSampleTypeIndex(1)
				return pp
			}(),
			wantErr: assert.Error,
		},
		{
			name: "invalid comment string index",
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append("")
				return dic
			}(),
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				st := pp.SampleType().AppendEmpty()
				st.SetAggregationTemporality(pprofile.AggregationTemporalityDelta)
				pp.PeriodType().SetAggregationTemporality(pprofile.AggregationTemporalityDelta)
				s := pp.Sample().AppendEmpty()
				s.Value().Append(0)
				pp.SetDefaultSampleTypeIndex(0)
				pp.CommentStrindices().Append(1)
				return pp
			}(),
			wantErr: assert.Error,
		},
		{
			name: "invalid attribute index",
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append("")
				return dic
			}(),
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				st := pp.SampleType().AppendEmpty()
				st.SetAggregationTemporality(pprofile.AggregationTemporalityDelta)
				pp.PeriodType().SetAggregationTemporality(pprofile.AggregationTemporalityDelta)
				s := pp.Sample().AppendEmpty()
				s.Value().Append(0)
				pp.SetDefaultSampleTypeIndex(0)
				pp.CommentStrindices().Append(0)
				pp.AttributeIndices().Append(1)
				return pp
			}(),
			wantErr: assert.Error,
		},
		{
			name: "invalid attribute unit index",
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append("")
				au := dic.AttributeUnits().AppendEmpty()
				au.SetAttributeKeyStrindex(1)
				return dic
			}(),
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				st := pp.SampleType().AppendEmpty()
				st.SetAggregationTemporality(pprofile.AggregationTemporalityDelta)
				pp.PeriodType().SetAggregationTemporality(pprofile.AggregationTemporalityDelta)
				s := pp.Sample().AppendEmpty()
				s.Value().Append(0)
				pp.SetDefaultSampleTypeIndex(0)
				pp.CommentStrindices().Append(0)
				pp.AttributeIndices().Append(0)
				return pp
			}(),
			wantErr: assert.Error,
		},
		{
			name: "valid",
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append("")
				au := dic.AttributeUnits().AppendEmpty()
				au.SetAttributeKeyStrindex(0)
				return dic
			}(),
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				st := pp.SampleType().AppendEmpty()
				st.SetAggregationTemporality(pprofile.AggregationTemporalityDelta)
				pp.PeriodType().SetAggregationTemporality(pprofile.AggregationTemporalityDelta)
				s := pp.Sample().AppendEmpty()
				s.Value().Append(0)
				pp.SetDefaultSampleTypeIndex(0)
				pp.CommentStrindices().Append(0)
				pp.AttributeIndices().Append(0)
				return pp
			}(),
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantErr(t, ValidateProfile(tt.dictionary, tt.profile))
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
		name       string
		dictionary pprofile.ProfilesDictionary
		profile    pprofile.Profile
		wantErr    assert.ErrorAssertionFunc
	}{
		{
			name:       "empty",
			dictionary: pprofile.NewProfilesDictionary(),
			profile:    pprofile.NewProfile(),
			wantErr:    assert.NoError,
		},
		{
			name: "valid",
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append("")
				return dic
			}(),
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
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
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append("")
				return dic
			}(),
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
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
			tt.wantErr(t, validateSampleType(tt.dictionary, tt.profile))
		})
	}
}

func Test_validateValueType(t *testing.T) {
	tests := []struct {
		name       string
		dictionary pprofile.ProfilesDictionary
		valueType  pprofile.ValueType
		wantErr    assert.ErrorAssertionFunc
	}{
		{
			name:       "type string index out of range",
			dictionary: pprofile.NewProfilesDictionary(),
			valueType:  pprofile.NewValueType(),
			wantErr:    assert.Error,
		},
		{
			name: "invalid aggregation temporality",
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append("")
				return dic
			}(),
			valueType: pprofile.NewValueType(),
			wantErr:   assert.Error,
		},
		{
			name: "unit string index out of range",
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append("")
				return dic
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
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append("")
				return dic
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
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append("")
				return dic
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
			tt.wantErr(t, validateValueType(tt.dictionary.StringTable().Len(), tt.valueType))
		})
	}
}

func Test_validateSamples(t *testing.T) {
	tests := []struct {
		name       string
		dictionary pprofile.ProfilesDictionary
		profile    pprofile.Profile
		wantErr    assert.ErrorAssertionFunc
	}{
		{
			name:       "no samples",
			dictionary: pprofile.NewProfilesDictionary(),
			profile:    pprofile.NewProfile(),
			wantErr:    assert.NoError,
		},
		{
			name:       "valid samples",
			dictionary: pprofile.NewProfilesDictionary(),
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.Sample().AppendEmpty()
				pp.Sample().AppendEmpty()
				return pp
			}(),
			wantErr: assert.NoError,
		},
		{
			name:       "invalid sample",
			dictionary: pprofile.NewProfilesDictionary(),
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
			tt.wantErr(t, validateSamples(tt.dictionary, tt.profile))
		})
	}
}

func Test_validateSample(t *testing.T) {
	tests := []struct {
		name       string
		dictionary pprofile.ProfilesDictionary
		profile    pprofile.Profile
		sample     pprofile.Sample
		wantErr    assert.ErrorAssertionFunc
	}{
		{
			name:       "empty",
			dictionary: pprofile.NewProfilesDictionary(),
			profile:    pprofile.NewProfile(),
			sample:     pprofile.NewSample(),
			wantErr:    assert.NoError,
		},
		{
			name:       "negative location length",
			dictionary: pprofile.NewProfilesDictionary(),
			profile:    pprofile.NewProfile(),
			sample: func() pprofile.Sample {
				s := pprofile.NewSample()
				s.SetLocationsLength(-1)
				return s
			}(),
			wantErr: assert.Error,
		},
		{
			name:       "location length out of range",
			dictionary: pprofile.NewProfilesDictionary(),
			profile:    pprofile.NewProfile(),
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
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.LocationTable().AppendEmpty()
				return dic
			}(),
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
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
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.LocationTable().AppendEmpty()
				return dic
			}(),
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
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
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.LocationTable().AppendEmpty()
				return dic
			}(),
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
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
			name:       "sample type length does not match",
			dictionary: pprofile.NewProfilesDictionary(),
			profile:    pprofile.NewProfile(),
			sample: func() pprofile.Sample {
				s := pprofile.NewSample()
				s.Value().Append(123)
				return s
			}(),
			wantErr: assert.Error,
		},
		{
			name: "attribute in range",
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.AttributeTable().AppendEmpty()
				return dic
			}(),
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
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
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.AttributeTable().AppendEmpty()
				return dic
			}(),
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
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
			name:       "timestamp in range",
			dictionary: pprofile.NewProfilesDictionary(),
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
			name:       "timestamp too small",
			dictionary: pprofile.NewProfilesDictionary(),
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
			name:       "timestamp too high",
			dictionary: pprofile.NewProfilesDictionary(),
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
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.LinkTable().AppendEmpty()
				return dic
			}(),
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
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
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.LinkTable().AppendEmpty()
				return dic
			}(),
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
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
			tt.wantErr(t, validateSample(tt.dictionary, tt.profile, tt.sample))
		})
	}
}

func Test_validateLocation(t *testing.T) {
	tests := []struct {
		name       string
		dictionary pprofile.ProfilesDictionary
		profile    pprofile.Profile
		location   pprofile.Location
		wantErr    assert.ErrorAssertionFunc
	}{
		{
			name:       "empty",
			dictionary: pprofile.NewProfilesDictionary(),
			location:   pprofile.NewLocation(),
			wantErr:    assert.NoError,
		},
		{
			name: "mapping index in range",
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append("")
				dic.MappingTable().AppendEmpty()
				return dic
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
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append("")
				dic.MappingTable().AppendEmpty()
				dic.AttributeTable().AppendEmpty()
				dic.FunctionTable().AppendEmpty()
				return dic
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
			tt.wantErr(t, validateLocation(tt.dictionary, tt.location))
		})
	}
}

func Test_validateLine(t *testing.T) {
	tests := []struct {
		name       string
		dictionary pprofile.ProfilesDictionary
		line       pprofile.Line
		wantErr    assert.ErrorAssertionFunc
	}{
		{
			name:       "function index out of range",
			dictionary: pprofile.NewProfilesDictionary(),
			line:       pprofile.NewLine(),
			wantErr:    assert.Error,
		},
		{
			name: "function index in range",
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append("")
				dic.FunctionTable().AppendEmpty()
				return dic
			}(),
			line:    pprofile.NewLine(),
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantErr(t, validateLine(tt.dictionary, tt.line))
		})
	}
}

func Test_validateMapping(t *testing.T) {
	tests := []struct {
		name       string
		dictionary pprofile.ProfilesDictionary
		mapping    pprofile.Mapping
		wantErr    assert.ErrorAssertionFunc
	}{
		{
			name:       "filename index out of range",
			dictionary: pprofile.NewProfilesDictionary(),
			mapping:    pprofile.NewMapping(),
			wantErr:    assert.Error,
		},
		{
			name: "filename index in range",
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append("")
				return dic
			}(),
			mapping: pprofile.NewMapping(),
			wantErr: assert.NoError,
		},
		{
			name: "attribute out of range",
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append("")
				return dic
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
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append("")
				dic.AttributeTable().AppendEmpty()
				return dic
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
			tt.wantErr(t, validateMapping(tt.dictionary, tt.mapping))
		})
	}
}

func Test_validateAttributeUnits(t *testing.T) {
	tests := []struct {
		name       string
		dictionary pprofile.ProfilesDictionary
		wantErr    assert.ErrorAssertionFunc
	}{
		{
			name:       "empty",
			dictionary: pprofile.NewProfilesDictionary(),
			wantErr:    assert.NoError,
		},
		{
			name: "in range",
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append("")
				dic.AttributeUnits().AppendEmpty()
				return dic
			}(),
			wantErr: assert.NoError,
		},
		{
			name: "unit index out of range",
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append("")
				au := dic.AttributeUnits().AppendEmpty()
				au.SetUnitStrindex(1)
				return dic
			}(),
			wantErr: assert.Error,
		},
		{
			name: "attribute key index out of range",
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append("")
				au := dic.AttributeUnits().AppendEmpty()
				au.SetAttributeKeyStrindex(1)
				return dic
			}(),
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantErr(t, validateAttributeUnits(tt.dictionary))
		})
	}
}

func Test_validateAttributeUnitAt(t *testing.T) {
	tests := []struct {
		name       string
		dictionary pprofile.ProfilesDictionary
		attrUnit   pprofile.AttributeUnit
		wantErr    assert.ErrorAssertionFunc
	}{
		{
			name:       "out of range",
			dictionary: pprofile.NewProfilesDictionary(),
			attrUnit:   pprofile.NewAttributeUnit(),
			wantErr:    assert.Error,
		},
		{
			name: "in range",
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append("")
				return dic
			}(),
			attrUnit: pprofile.NewAttributeUnit(),
			wantErr:  assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantErr(t, validateAttributeUnit(tt.dictionary, tt.attrUnit))
		})
	}
}
