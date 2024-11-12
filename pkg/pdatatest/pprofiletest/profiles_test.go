// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofiletest

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pprofile"
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
	}{}
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
	}{}
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
	}{}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.err, CompareProfileLinkSlice(test.expected, test.actual))
		})
	}
}
