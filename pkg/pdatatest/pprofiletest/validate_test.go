// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofiletest

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/testdata"
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
			wantErr: assert.NoError,
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
				s := pp.Samples().AppendEmpty()
				s.SetLinkIndex(42)
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
				s := pp.Samples().AppendEmpty()
				s.Values().Append(0)
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
				au := dic.AttributeTable().AppendEmpty()
				au.SetKeyStrindex(1)
				return dic
			}(),
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				s := pp.Samples().AppendEmpty()
				s.Values().Append(0)
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
				au := dic.AttributeTable().AppendEmpty()
				au.SetKeyStrindex(0)
				return dic
			}(),
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				s := pp.Samples().AppendEmpty()
				s.Values().Append(0)
				pp.AttributeIndices().Append(0)
				return pp
			}(),
			wantErr: assert.NoError,
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
			wantErr:    assert.Error,
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
				pp.SampleType().SetTypeStrindex(3)
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
			name: "unit string index out of range",
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append("")
				return dic
			}(),
			valueType: func() pprofile.ValueType {
				pp := pprofile.NewValueType()
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
				pp.Samples().AppendEmpty()
				pp.Samples().AppendEmpty()
				return pp
			}(),
			wantErr: assert.NoError,
		},
		{
			name:       "invalid sample",
			dictionary: pprofile.NewProfilesDictionary(),
			profile: func() pprofile.Profile {
				pp := pprofile.NewProfile()
				pp.Samples().AppendEmpty()
				s := pp.Samples().AppendEmpty()
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
				l.Lines().AppendEmpty()
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

func Test_validateKeyValueAndUnitsUnits(t *testing.T) {
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
				dic.AttributeTable().AppendEmpty()
				return dic
			}(),
			wantErr: assert.NoError,
		},
		{
			name: "unit index out of range",
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append("")
				au := dic.AttributeTable().AppendEmpty()
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
				au := dic.AttributeTable().AppendEmpty()
				au.SetKeyStrindex(1)
				return dic
			}(),
			wantErr: assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantErr(t, validateKeyValueAndUnits(tt.dictionary))
		})
	}
}

func Test_validateKeyValueAndUnit(t *testing.T) {
	tests := []struct {
		name       string
		dictionary pprofile.ProfilesDictionary
		attrUnit   pprofile.KeyValueAndUnit
		wantErr    assert.ErrorAssertionFunc
	}{
		{
			name:       "out of range",
			dictionary: pprofile.NewProfilesDictionary(),
			attrUnit:   pprofile.NewKeyValueAndUnit(),
			wantErr:    assert.Error,
		},
		{
			name: "in range",
			dictionary: func() pprofile.ProfilesDictionary {
				dic := pprofile.NewProfilesDictionary()
				dic.StringTable().Append("")
				return dic
			}(),
			attrUnit: pprofile.NewKeyValueAndUnit(),
			wantErr:  assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.wantErr(t, validateKeyValueAndUnit(tt.dictionary, tt.attrUnit))
		})
	}
}

func TestGeneratedData(t *testing.T) {
	data := testdata.GenerateProfiles(3)

	for i := range data.ResourceProfiles().Len() {
		rp := data.ResourceProfiles().At(i)
		for j := range rp.ScopeProfiles().Len() {
			sp := rp.ScopeProfiles().At(j)
			for k := range sp.Profiles().Len() {
				p := sp.Profiles().At(k)
				err := ValidateProfile(data.Dictionary(), p)
				if err != nil {
					err = fmt.Errorf("profile ResourceProfiles[%d]ScopeProfiles[%d]Profiles[%d]: %w", i, j, k, err)
				}
				require.NoError(t, err)
			}
		}
	}
}
