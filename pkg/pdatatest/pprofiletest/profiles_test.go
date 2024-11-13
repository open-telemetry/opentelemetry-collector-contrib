// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package pprofiletest

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	}{
		{
			name: "empty",
			expected: func() pprofile.Profiles {
				p := pprofile.NewProfiles()
				return p
			}(),
			actual: func() pprofile.Profiles {
				p := pprofile.NewProfiles()
				return p
			}(),
		},
		{
			name: "equal",
			expected: func() pprofile.Profiles {
				p := pprofile.NewProfiles()
				rl := p.ResourceProfiles().AppendEmpty()
				rl.Resource().Attributes().PutStr("key1", "value1")
				l := rl.ScopeProfiles().AppendEmpty().Profiles().AppendEmpty()
				l.Attributes().PutStr("scope-attr1", "value1")
				l.SetProfileID(pprofile.NewProfileIDEmpty())
				return p
			}(),
			actual: func() pprofile.Profiles {
				p := pprofile.NewProfiles()
				rl := p.ResourceProfiles().AppendEmpty()
				rl.Resource().Attributes().PutStr("key1", "value1")
				l := rl.ScopeProfiles().AppendEmpty().Profiles().AppendEmpty()
				l.Attributes().PutStr("scope-attr1", "value1")
				l.SetProfileID(pprofile.NewProfileIDEmpty())
				return p
			}(),
		},
		{
			name: "resource order",
			expected: func() pprofile.Profiles {
				p := pprofile.NewProfiles()
				rl := p.ResourceProfiles().AppendEmpty()
				rl.Resource().Attributes().PutStr("key1", "value1")
				l := rl.ScopeProfiles().AppendEmpty().Profiles().AppendEmpty()
				l.Attributes().PutStr("scope-attr1", "value1")
				l.SetProfileID(pprofile.NewProfileIDEmpty())
				rl2 := p.ResourceProfiles().AppendEmpty()
				rl2.Resource().Attributes().PutStr("key2", "value2")
				l2 := rl2.ScopeProfiles().AppendEmpty().Profiles().AppendEmpty()
				l2.Attributes().PutStr("scope-attr2", "value2")
				l2.SetProfileID(pprofile.NewProfileIDEmpty())
				return p
			}(),
			actual: func() pprofile.Profiles {
				p := pprofile.NewProfiles()
				rl2 := p.ResourceProfiles().AppendEmpty()
				rl2.Resource().Attributes().PutStr("key2", "value2")
				l2 := rl2.ScopeProfiles().AppendEmpty().Profiles().AppendEmpty()
				l2.Attributes().PutStr("scope-attr2", "value2")
				l2.SetProfileID(pprofile.NewProfileIDEmpty())
				rl := p.ResourceProfiles().AppendEmpty()
				rl.Resource().Attributes().PutStr("key1", "value1")
				l := rl.ScopeProfiles().AppendEmpty().Profiles().AppendEmpty()
				l.Attributes().PutStr("scope-attr1", "value1")
				l.SetProfileID(pprofile.NewProfileIDEmpty())
				return p
			}(),
			withoutOptions: multierr.Combine(
				errors.New(`resources are out of order: resource "map[key1:value1]" expected at index 0, found at index 1`),
				errors.New(`resources are out of order: resource "map[key2:value2]" expected at index 1, found at index 0`),
			),
			compareOptions: []CompareProfilesOption{
				IgnoreResourceProfilesOrder()},
		},
		{
			name: "resource masked attribute",
			expected: func() pprofile.Profiles {
				p := pprofile.NewProfiles()
				rl := p.ResourceProfiles().AppendEmpty()
				rl.Resource().Attributes().PutStr("key1", "value1")
				return p
			}(),
			actual: func() pprofile.Profiles {
				p := pprofile.NewProfiles()
				rl := p.ResourceProfiles().AppendEmpty()
				rl.Resource().Attributes().PutStr("key1", "value2")
				return p
			}(),
			withoutOptions: multierr.Combine(
				errors.New(`missing expected resource: map[key1:value1]`),
				errors.New(`unexpected resource: map[key1:value2]`),
			),
			compareOptions: []CompareProfilesOption{
				IgnoreResourceAttributeValue("key1")},
		},
		{
			name: "resource scope order",
			expected: func() pprofile.Profiles {
				p := pprofile.NewProfiles()
				rl := p.ResourceProfiles().AppendEmpty()
				rl.Resource().Attributes().PutStr("key1", "value1")
				l := rl.ScopeProfiles().AppendEmpty().Scope()
				l.SetName("scope1")
				l.Attributes().PutStr("scope-attr1", "value1")
				l2 := rl.ScopeProfiles().AppendEmpty().Scope()
				l2.Attributes().PutStr("scope-attr2", "value2")
				l2.SetName("scope2")
				return p
			}(),
			actual: func() pprofile.Profiles {
				p := pprofile.NewProfiles()
				rl := p.ResourceProfiles().AppendEmpty()
				rl.Resource().Attributes().PutStr("key1", "value1")
				l2 := rl.ScopeProfiles().AppendEmpty().Scope()
				l2.Attributes().PutStr("scope-attr2", "value2")
				l2.SetName("scope2")
				l := rl.ScopeProfiles().AppendEmpty().Scope()
				l.Attributes().PutStr("scope-attr1", "value1")
				l.SetName("scope1")
				return p
			}(),
			withoutOptions: errors.New(`resource "map[key1:value1]": scopes are out of order: scope scope1 expected at index 0, found at index 1; resource "map[key1:value1]": scopes are out of order: scope scope2 expected at index 1, found at index 0`),
			compareOptions: []CompareProfilesOption{
				IgnoreScopeProfilesOrder()},
		},
		{
			name: "mask scope attribute",
			expected: func() pprofile.Profiles {
				p := pprofile.NewProfiles()
				rl := p.ResourceProfiles().AppendEmpty()
				rl.Resource().Attributes().PutStr("key1", "value1")
				l := rl.ScopeProfiles().AppendEmpty().Scope()
				l.SetName("scope1")
				l.Attributes().PutStr("scope-attr1", "value1")
				l2 := rl.ScopeProfiles().AppendEmpty().Scope()
				l2.Attributes().PutStr("scope-attr2", "value2")
				l2.SetName("scope2")
				return p
			}(),
			actual: func() pprofile.Profiles {
				p := pprofile.NewProfiles()
				rl := p.ResourceProfiles().AppendEmpty()
				rl.Resource().Attributes().PutStr("key1", "value1")
				l := rl.ScopeProfiles().AppendEmpty().Scope()
				l.Attributes().PutStr("scope-attr1", "value12")
				l.SetName("scope1")
				l2 := rl.ScopeProfiles().AppendEmpty().Scope()
				l2.Attributes().PutStr("scope-attr2", "value22")
				l2.SetName("scope2")
				return p
			}(),
			withoutOptions: errors.New(`resource "map[key1:value1]": scope "scope1": attributes don't match expected: map[scope-attr1:value1], actual: map[scope-attr1:value12]; resource "map[key1:value1]": scope "scope2": attributes don't match expected: map[scope-attr2:value2], actual: map[scope-attr2:value22]`),
			compareOptions: []CompareProfilesOption{
				IgnoreScopeAttributeValue("scope-attr2"),
				IgnoreScopeAttributeValue("scope-attr1"),
			},
		},

		// TODO
		// ignore scope attribute value
		// ignore container order

		// ignore profilecontainer attribute value
		// ignore profilecontainer attribute order
		// ignore profilecontainer timestamp value

		// ignore profile attribute value
		// ignore profile attribute order
		// ignore profile timestamp value

	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := CompareProfiles(tc.expected, tc.actual)
			if tc.withoutOptions == nil {
				assert.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.withoutOptions.Error())
			}

			if tc.compareOptions == nil {
				return
			}

			err = CompareProfiles(tc.expected, tc.actual, tc.compareOptions...)
			if tc.withOptions == nil {
				assert.NoError(t, err)
			} else {
				require.EqualError(t, err, tc.withOptions.Error())
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
				l.Attributes().PutStr("scope-attr1", "value1")
				l.SetProfileID(pprofile.NewProfileIDEmpty())
				return rl
			}(),
			actual: func() pprofile.ResourceProfiles {
				rl := pprofile.NewResourceProfiles()
				rl.Resource().Attributes().PutStr("key1", "value1")
				l := rl.ScopeProfiles().AppendEmpty().Profiles().AppendEmpty()
				l.Attributes().PutStr("scope-attr1", "value1")
				l.SetProfileID(pprofile.NewProfileIDEmpty())
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
			require.Equal(t, test.err, CompareResourceProfiles(test.expected, test.actual))
		})
	}
}

func TestCompareScopeProfiles(t *testing.T) {
	tests := []struct {
		name     string
		expected pprofile.ScopeProfiles
		actual   pprofile.ScopeProfiles
		err      error
	}{
		{
			name: "equal",
			expected: func() pprofile.ScopeProfiles {
				sl := pprofile.NewScopeProfiles()
				sl.Scope().SetName("scope-name")
				l := sl.Profiles().AppendEmpty()
				l.Attributes().PutStr("scope-attr1", "value1")
				l.SetProfileID(pprofile.NewProfileIDEmpty())
				return sl
			}(),
			actual: func() pprofile.ScopeProfiles {
				sl := pprofile.NewScopeProfiles()
				sl.Scope().SetName("scope-name")
				l := sl.Profiles().AppendEmpty()
				l.Attributes().PutStr("scope-attr1", "value1")
				l.SetProfileID(pprofile.NewProfileIDEmpty())
				return sl
			}(),
		},
		{
			name: "scope-name-mismatch",
			expected: func() pprofile.ScopeProfiles {
				sl := pprofile.NewScopeProfiles()
				sl.Scope().SetName("scope-name")
				return sl
			}(),
			actual: func() pprofile.ScopeProfiles {
				sl := pprofile.NewScopeProfiles()
				sl.Scope().SetName("scope-name-2")
				return sl
			}(),
			err: errors.New("name doesn't match expected: scope-name, actual: scope-name-2"),
		},
		{
			name: "scope-version-mismatch",
			expected: func() pprofile.ScopeProfiles {
				sl := pprofile.NewScopeProfiles()
				sl.Scope().SetVersion("scope-version")
				return sl
			}(),
			actual: func() pprofile.ScopeProfiles {
				sl := pprofile.NewScopeProfiles()
				sl.Scope().SetVersion("scope-version-2")
				return sl
			}(),
			err: errors.New("version doesn't match expected: scope-version, actual: scope-version-2"),
		},
		{
			name: "scope-attributes-mismatch",
			expected: func() pprofile.ScopeProfiles {
				sl := pprofile.NewScopeProfiles()
				sl.Scope().Attributes().PutStr("scope-attr1", "value1")
				sl.Scope().Attributes().PutStr("scope-attr2", "value2")
				return sl
			}(),
			actual: func() pprofile.ScopeProfiles {
				sl := pprofile.NewScopeProfiles()
				sl.Scope().Attributes().PutStr("scope-attr1", "value1")
				sl.Scope().SetDroppedAttributesCount(1)
				return sl
			}(),
			err: multierr.Combine(
				errors.New("attributes don't match expected: map[scope-attr1:value1 scope-attr2:value2], "+
					"actual: map[scope-attr1:value1]"),
				errors.New("dropped attributes count doesn't match expected: 0, actual: 1"),
			),
		},
		{
			name: "scope-schema-url-mismatch",
			expected: func() pprofile.ScopeProfiles {
				rl := pprofile.NewScopeProfiles()
				rl.SetSchemaUrl("schema-url")
				return rl
			}(),
			actual: func() pprofile.ScopeProfiles {
				rl := pprofile.NewScopeProfiles()
				rl.SetSchemaUrl("schema-url-2")
				return rl
			}(),
			err: errors.New("schema url doesn't match expected: schema-url, actual: schema-url-2"),
		},
		{
			name: "profiles-number-mismatch",
			expected: func() pprofile.ScopeProfiles {
				sl := pprofile.NewScopeProfiles()
				sl.Profiles().AppendEmpty()
				sl.Profiles().AppendEmpty()
				return sl
			}(),
			actual: func() pprofile.ScopeProfiles {
				sl := pprofile.NewScopeProfiles()
				sl.Profiles().AppendEmpty()
				return sl
			}(),
			err: errors.New("number of profile containers doesn't match expected: 2, actual: 1"),
		},
		{
			name: "profile-records-order-mismatch",
			expected: func() pprofile.ScopeProfiles {
				sl := pprofile.NewScopeProfiles()
				l := sl.Profiles().AppendEmpty()
				l.Attributes().PutStr("scope-attr1", "value1")
				l.SetProfileID(pprofile.ProfileID([]byte("profileid1111111")))
				l = sl.Profiles().AppendEmpty()
				l.Attributes().PutStr("scope-attr2", "value2")
				l.SetProfileID(pprofile.ProfileID([]byte("profileid1111112")))
				return sl
			}(),
			actual: func() pprofile.ScopeProfiles {
				sl := pprofile.NewScopeProfiles()
				l := sl.Profiles().AppendEmpty()
				l.Attributes().PutStr("scope-attr2", "value2")
				l.SetProfileID(pprofile.ProfileID([]byte("profileid1111111")))
				l = sl.Profiles().AppendEmpty()
				l.Attributes().PutStr("scope-attr1", "value1")
				l.SetProfileID(pprofile.ProfileID([]byte("profileid1111113")))
				return sl
			}(),
			err: multierr.Combine(
				errors.New(`profile containers are out of order: profile container "map[scope-attr1:value1]" expected at index 0, found at index 1`),
				errors.New(`profile containers are out of order: profile container "map[scope-attr2:value2]" expected at index 1, found at index 0`),
			),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.err, CompareScopeProfiles(test.expected, test.actual))
		})
	}
}

func TestCompareProfileContainer(t *testing.T) {
	tests := []struct {
		name     string
		expected pprofile.ProfileContainer
		actual   pprofile.ProfileContainer
		err      error
	}{
		{
			name: "empty",
			expected: func() pprofile.ProfileContainer {
				l := pprofile.NewProfileContainer()
				return l
			}(),
			actual: func() pprofile.ProfileContainer {
				l := pprofile.NewProfileContainer()
				return l
			}(),
		},
		{
			name: "equal",
			expected: func() pprofile.ProfileContainer {
				l := pprofile.NewProfileContainer()
				l.SetProfileID(pprofile.ProfileID([]byte("profileid1111111")))
				l.SetDroppedAttributesCount(2)
				p := l.Profile()
				p.SetKeepFrames(1)
				return l
			}(),
			actual: func() pprofile.ProfileContainer {
				l := pprofile.NewProfileContainer()
				l.SetProfileID(pprofile.ProfileID([]byte("profileid1111111")))
				l.SetDroppedAttributesCount(2)
				p := l.Profile()
				p.SetKeepFrames(1)
				return l
			}(),
		},
		{
			name: "not equal",
			expected: func() pprofile.ProfileContainer {
				l := pprofile.NewProfileContainer()
				l.SetProfileID(pprofile.ProfileID([]byte("profileid1111111")))
				l.SetDroppedAttributesCount(2)
				p := l.Profile()
				p.SetKeepFrames(1)
				return l
			}(),
			actual: func() pprofile.ProfileContainer {
				l := pprofile.NewProfileContainer()
				l.SetProfileID(pprofile.ProfileID([]byte("profileid1111112")))
				l.SetDroppedAttributesCount(3)
				p := l.Profile()
				p.SetKeepFrames(3)
				return l
			}(),
			err: multierr.Combine(
				errors.New(`dropped attributes count doesn't match expected: 2, actual: 3`),
				errors.New(`profileID does not match expected '70726f66696c65696431313131313131', actual '70726f66696c65696431313131313132'`),
				fmt.Errorf(`profile "map[]": %w`, fmt.Errorf(`keepFrames does not match expected '1', actual '3'`)),
			),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.err, CompareProfileContainer(test.expected, test.actual))
		})
	}
}

func TestCompareProfile(t *testing.T) {
	tests := []struct {
		name     string
		expected pprofile.Profile
		actual   pprofile.Profile
		err      error
	}{
		{
			name: "empty",
			expected: func() pprofile.Profile {
				l := pprofile.NewProfile()
				return l
			}(),
			actual: func() pprofile.Profile {
				l := pprofile.NewProfile()
				return l
			}(),
		},
		{
			name: "equal",
			expected: func() pprofile.Profile {
				l := pprofile.NewProfile()
				l.SetKeepFrames(1)
				l.AttributeTable().PutStr("key", "val")
				l.SetPeriod(1)
				s := l.SampleType().AppendEmpty()
				s.SetType(1)
				s.SetUnit(1)
				a := l.AttributeUnits().AppendEmpty()
				a.SetAttributeKey(1)
				a.SetUnit(1)
				return l
			}(),
			actual: func() pprofile.Profile {
				l := pprofile.NewProfile()
				l.SetKeepFrames(1)
				l.AttributeTable().PutStr("key", "val")
				l.SetPeriod(1)
				s := l.SampleType().AppendEmpty()
				s.SetType(1)
				s.SetUnit(1)
				a := l.AttributeUnits().AppendEmpty()
				a.SetAttributeKey(1)
				a.SetUnit(1)
				return l
			}(),
		},
		{
			name: "not equal",
			expected: func() pprofile.Profile {
				l := pprofile.NewProfile()
				l.SetKeepFrames(1)
				l.AttributeTable().PutStr("key", "val")
				l.SetPeriod(1)
				s := l.SampleType().AppendEmpty()
				s.SetType(1)
				s.SetUnit(1)
				a := l.AttributeUnits().AppendEmpty()
				a.SetAttributeKey(1)
				a.SetUnit(1)
				return l
			}(),
			actual: func() pprofile.Profile {
				l := pprofile.NewProfile()
				l.SetKeepFrames(2)
				l.AttributeTable().PutStr("key1", "val1")
				l.SetPeriod(2)
				s := l.SampleType().AppendEmpty()
				s.SetType(2)
				s.SetUnit(2)
				a := l.AttributeUnits().AppendEmpty()
				a.SetAttributeKey(2)
				a.SetUnit(2)
				return l
			}(),
			err: multierr.Combine(
				errors.New(`attributes don't match expected: map[key:val], actual: map[key1:val1]`),
				errors.New(`keepFrames does not match expected '1', actual '2'`),
				errors.New(`period does not match expected '1', actual '2'`),
				fmt.Errorf(`sampleType: %w`, fmt.Errorf(`missing expected valueType "unit: 1, type: 1, aggregationTemporality: 0"`)),
				fmt.Errorf(`sampleType: %w`, fmt.Errorf(`unexpected valueType "unit: 2, type: 2, aggregationTemporality: 0"`)),
				fmt.Errorf(`attributeUnits: %w`, fmt.Errorf(`missing expected attributeUnit "attributeKey: 1"`)),
				fmt.Errorf(`attributeUnits: %w`, fmt.Errorf(`unexpected profile attributeUnit "attributeKey: 2"`)),
			),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.err, CompareProfile(test.expected, test.actual))
		})
	}
}

func TestCompareProfileValueTypeSlice(t *testing.T) {
	tests := []struct {
		name     string
		expected pprofile.ValueTypeSlice
		actual   pprofile.ValueTypeSlice
		err      error
	}{
		{
			name: "empty",
			expected: func() pprofile.ValueTypeSlice {
				l := pprofile.NewValueTypeSlice()
				return l
			}(),
			actual: func() pprofile.ValueTypeSlice {
				l := pprofile.NewValueTypeSlice()
				return l
			}(),
		},
		{
			name: "equal",
			expected: func() pprofile.ValueTypeSlice {
				l := pprofile.NewValueTypeSlice()
				i1 := l.AppendEmpty()
				i1.SetType(1)
				i1.SetUnit(1)
				i1.SetAggregationTemporality(1)
				i2 := l.AppendEmpty()
				i2.SetType(2)
				i2.SetUnit(2)
				i2.SetAggregationTemporality(1)
				return l
			}(),
			actual: func() pprofile.ValueTypeSlice {
				l := pprofile.NewValueTypeSlice()
				i1 := l.AppendEmpty()
				i1.SetType(1)
				i1.SetUnit(1)
				i1.SetAggregationTemporality(1)
				i2 := l.AppendEmpty()
				i2.SetType(2)
				i2.SetUnit(2)
				i2.SetAggregationTemporality(1)
				return l
			}(),
		},
		{
			name: "equal wrong order",
			expected: func() pprofile.ValueTypeSlice {
				l := pprofile.NewValueTypeSlice()
				i1 := l.AppendEmpty()
				i1.SetType(1)
				i1.SetUnit(1)
				i1.SetAggregationTemporality(1)
				i2 := l.AppendEmpty()
				i2.SetType(2)
				i2.SetUnit(2)
				i2.SetAggregationTemporality(1)
				return l
			}(),
			actual: func() pprofile.ValueTypeSlice {
				l := pprofile.NewValueTypeSlice()
				i2 := l.AppendEmpty()
				i2.SetType(2)
				i2.SetUnit(2)
				i2.SetAggregationTemporality(1)
				i1 := l.AppendEmpty()
				i1.SetType(1)
				i1.SetUnit(1)
				i1.SetAggregationTemporality(1)
				return l
			}(),
			err: multierr.Combine(
				errors.New(`valueTypes are out of order: valueType "unit: 1, type: 1, aggregationTemporality: 1" expected at index 0, found at index 1`),
				errors.New(`valueTypes are out of order: valueType "unit: 2, type: 2, aggregationTemporality: 1" expected at index 1, found at index 0`),
			),
		},
		{
			name: "wrong length",
			expected: func() pprofile.ValueTypeSlice {
				l := pprofile.NewValueTypeSlice()
				i1 := l.AppendEmpty()
				i1.SetType(1)
				i1.SetUnit(1)
				i1.SetAggregationTemporality(1)
				return l
			}(),
			actual: func() pprofile.ValueTypeSlice {
				l := pprofile.NewValueTypeSlice()
				i1 := l.AppendEmpty()
				i1.SetType(1)
				i1.SetUnit(1)
				i1.SetAggregationTemporality(1)
				i2 := l.AppendEmpty()
				i2.SetType(2)
				i2.SetUnit(2)
				i2.SetAggregationTemporality(1)
				return l
			}(),
			err: multierr.Combine(
				errors.New(`number of valueTypes doesn't match expected: 1, actual: 2`),
			),
		},
		{
			name: "not equal - does not match expected",
			expected: func() pprofile.ValueTypeSlice {
				l := pprofile.NewValueTypeSlice()
				i1 := l.AppendEmpty()
				i1.SetType(1)
				i1.SetUnit(1)
				i1.SetAggregationTemporality(1)
				i2 := l.AppendEmpty()
				i2.SetType(2)
				i2.SetUnit(2)
				i2.SetAggregationTemporality(1)
				return l
			}(),
			actual: func() pprofile.ValueTypeSlice {
				l := pprofile.NewValueTypeSlice()
				i1 := l.AppendEmpty()
				i1.SetType(1)
				i1.SetUnit(1)
				i1.SetAggregationTemporality(1)
				i2 := l.AppendEmpty()
				i2.SetType(2)
				i2.SetUnit(2)
				i2.SetAggregationTemporality(2)
				return l
			}(),
			err: multierr.Combine(
				errors.New(`expected valueType "unit: 2, type: 2, aggregationTemporality: 1",got "unit: 2, type: 2, aggregationTemporality: 2"`),
			),
		},
		{
			name: "not equal - missing",
			expected: func() pprofile.ValueTypeSlice {
				l := pprofile.NewValueTypeSlice()
				i1 := l.AppendEmpty()
				i1.SetType(1)
				i1.SetUnit(1)
				i1.SetAggregationTemporality(1)
				i2 := l.AppendEmpty()
				i2.SetType(2)
				i2.SetUnit(2)
				i2.SetAggregationTemporality(1)
				return l
			}(),
			actual: func() pprofile.ValueTypeSlice {
				l := pprofile.NewValueTypeSlice()
				i1 := l.AppendEmpty()
				i1.SetType(1)
				i1.SetUnit(1)
				i1.SetAggregationTemporality(1)
				i2 := l.AppendEmpty()
				i2.SetType(3)
				i2.SetUnit(3)
				i2.SetAggregationTemporality(1)
				return l
			}(),
			err: multierr.Combine(
				errors.New(`missing expected valueType "unit: 2, type: 2, aggregationTemporality: 1"`),
				errors.New(`unexpected valueType "unit: 3, type: 3, aggregationTemporality: 1"`),
			),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.err, CompareProfileValueTypeSlice(test.expected, test.actual))
		})
	}
}

func TestCompareProfileSampleSlice(t *testing.T) {
	tests := []struct {
		name     string
		expected pprofile.SampleSlice
		actual   pprofile.SampleSlice
		err      error
	}{
		{
			name: "empty",
			expected: func() pprofile.SampleSlice {
				l := pprofile.NewSampleSlice()
				return l
			}(),
			actual: func() pprofile.SampleSlice {
				l := pprofile.NewSampleSlice()
				return l
			}(),
		},
		{
			name: "equal",
			expected: func() pprofile.SampleSlice {
				l := pprofile.NewSampleSlice()
				i1 := l.AppendEmpty()
				i1.SetLink(1)
				i1.SetLocationsLength(1)
				i1.Attributes().Append(1, 2)
				i2 := l.AppendEmpty()
				i2.Attributes().Append(1, 2, 3)
				i2.SetLink(2)
				i2.SetLocationsLength(2)
				return l
			}(),
			actual: func() pprofile.SampleSlice {
				l := pprofile.NewSampleSlice()
				i1 := l.AppendEmpty()
				i1.SetLink(1)
				i1.SetLocationsLength(1)
				i1.Attributes().Append(1, 2)
				i2 := l.AppendEmpty()
				i2.Attributes().Append(1, 2, 3)
				i2.SetLink(2)
				i2.SetLocationsLength(2)
				return l
			}(),
		},
		{
			name: "equal wrong order",
			expected: func() pprofile.SampleSlice {
				l := pprofile.NewSampleSlice()
				i1 := l.AppendEmpty()
				i1.SetLink(1)
				i1.SetLocationsLength(1)
				i1.Attributes().Append(1, 2)
				i2 := l.AppendEmpty()
				i2.Attributes().Append(1, 2, 3)
				i2.SetLink(2)
				i2.SetLocationsLength(2)
				return l
			}(),
			actual: func() pprofile.SampleSlice {
				l := pprofile.NewSampleSlice()
				i2 := l.AppendEmpty()
				i2.Attributes().Append(1, 2, 3)
				i2.SetLink(2)
				i2.SetLocationsLength(2)
				i1 := l.AppendEmpty()
				i1.SetLink(1)
				i1.SetLocationsLength(1)
				i1.Attributes().Append(1, 2)
				return l
			}(),
			err: multierr.Combine(
				errors.New(`samples are out of order: sample "attributes: [1 2]" expected at index 0, found at index 1`),
				errors.New(`samples are out of order: sample "attributes: [1 2 3]" expected at index 1, found at index 0`),
			),
		},
		{
			name: "wrong length",
			expected: func() pprofile.SampleSlice {
				l := pprofile.NewSampleSlice()
				i1 := l.AppendEmpty()
				i1.SetLink(1)
				i1.SetLocationsLength(1)
				i1.Attributes().Append(1, 2)
				return l
			}(),
			actual: func() pprofile.SampleSlice {
				l := pprofile.NewSampleSlice()
				i1 := l.AppendEmpty()
				i1.SetLink(1)
				i1.SetLocationsLength(1)
				i1.Attributes().Append(1, 2)
				i2 := l.AppendEmpty()
				i2.Attributes().Append(1, 2, 3)
				i2.SetLink(2)
				i2.SetLocationsLength(2)
				return l
			}(),
			err: multierr.Combine(
				errors.New(`number of samples doesn't match expected: 1, actual: 2`),
			),
		},
		{
			name: "not equal - does not match expected",
			expected: func() pprofile.SampleSlice {
				l := pprofile.NewSampleSlice()
				i1 := l.AppendEmpty()
				i1.SetLink(1)
				i1.SetLocationsLength(1)
				i1.Attributes().Append(1, 2)
				i2 := l.AppendEmpty()
				i2.Attributes().Append(1, 2, 3)
				i2.SetLink(2)
				i2.SetLocationsLength(2)
				return l
			}(),
			actual: func() pprofile.SampleSlice {
				l := pprofile.NewSampleSlice()
				i1 := l.AppendEmpty()
				i1.SetLink(1)
				i1.SetLocationsLength(1)
				i1.Attributes().Append(1, 2)
				i2 := l.AppendEmpty()
				i2.Attributes().Append(1, 2, 3)
				i2.SetLink(3)
				i2.SetLocationsLength(3)
				return l
			}(),
			err: multierr.Combine(
				fmt.Errorf(`sample "attributes: [1 2 3]": %w`, fmt.Errorf(`expected locationLenght '2', got '3'`)),
				fmt.Errorf(`sample "attributes: [1 2 3]": %w`, fmt.Errorf(`expected link '2', got '3'`)),
			),
		},
		{
			name: "not equal - missing",
			expected: func() pprofile.SampleSlice {
				l := pprofile.NewSampleSlice()
				i1 := l.AppendEmpty()
				i1.SetLink(1)
				i1.SetLocationsLength(1)
				i1.Attributes().Append(1, 2)
				i2 := l.AppendEmpty()
				i2.Attributes().Append(1, 2, 3)
				i2.SetLink(2)
				i2.SetLocationsLength(2)
				return l
			}(),
			actual: func() pprofile.SampleSlice {
				l := pprofile.NewSampleSlice()
				i1 := l.AppendEmpty()
				i1.SetLink(1)
				i1.SetLocationsLength(1)
				i1.Attributes().Append(1, 2)
				i2 := l.AppendEmpty()
				i2.Attributes().Append(1, 2, 3, 5)
				i2.SetLink(3)
				i2.SetLocationsLength(3)
				return l
			}(),
			err: multierr.Combine(
				errors.New(`missing expected sample "attributes: [1 2 3]"`),
				errors.New(`unexpected sample "attributes: [1 2 3 5]"`),
			),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.err, CompareProfileSampleSlice(test.expected, test.actual))
		})
	}
}

func TestCompareProfileSample(t *testing.T) {
	tests := []struct {
		name     string
		expected pprofile.Sample
		actual   pprofile.Sample
		err      error
	}{
		{
			name: "empty",
			expected: func() pprofile.Sample {
				l := pprofile.NewSample()
				return l
			}(),
			actual: func() pprofile.Sample {
				l := pprofile.NewSample()
				return l
			}(),
		},
		{
			name: "equal",
			expected: func() pprofile.Sample {
				l := pprofile.NewSample()
				l.SetLocationsStartIndex(1)
				l.SetLocationsLength(1)
				l.SetStacktraceIdIndex(1)
				l.LocationIndex().Append(1, 2)
				l.Attributes().Append(1, 2)
				l.Label().AppendEmpty().SetKey(1)
				return l
			}(),
			actual: func() pprofile.Sample {
				l := pprofile.NewSample()
				l.SetLocationsStartIndex(1)
				l.SetLocationsLength(1)
				l.SetStacktraceIdIndex(1)
				l.LocationIndex().Append(1, 2)
				l.Attributes().Append(1, 2)
				l.Label().AppendEmpty().SetKey(1)
				return l
			}(),
		},
		{
			name: "not equal",
			expected: func() pprofile.Sample {
				l := pprofile.NewSample()
				l.SetLocationsStartIndex(1)
				l.SetLocationsLength(1)
				l.SetStacktraceIdIndex(1)
				l.LocationIndex().Append(1, 2)
				l.Attributes().Append(1, 2)
				l.Label().AppendEmpty().SetKey(1)
				return l
			}(),
			actual: func() pprofile.Sample {
				l := pprofile.NewSample()
				l.SetLocationsStartIndex(2)
				l.SetLocationsLength(3)
				l.SetStacktraceIdIndex(3)
				l.LocationIndex().Append(1, 2)
				l.Attributes().Append(1, 2, 3)
				l.Label().AppendEmpty().SetKey(2)
				return l
			}(),
			err: multierr.Combine(
				errors.New(`expected locationStartIndex '1', got '2'`),
				errors.New(`expected locationLenght '1', got '3'`),
				errors.New(`expected stacktraceIdIndex '1', got '3'`),
				errors.New(`expected attributes '[1 2]', got '[1 2 3]'`),
				fmt.Errorf(`labelSlice of sample with attributes "[1 2]": %w`, fmt.Errorf(`missing expected label "key: 1"`)),
				fmt.Errorf(`labelSlice of sample with attributes "[1 2]": %w`, fmt.Errorf(`unexpected label "key: 2"`)),
			),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.err, CompareProfileSample(test.expected, test.actual))
		})
	}
}

func TestCompareProfileLabelSlice(t *testing.T) {
	tests := []struct {
		name     string
		expected pprofile.LabelSlice
		actual   pprofile.LabelSlice
		err      error
	}{
		{
			name: "empty",
			expected: func() pprofile.LabelSlice {
				l := pprofile.NewLabelSlice()
				return l
			}(),
			actual: func() pprofile.LabelSlice {
				l := pprofile.NewLabelSlice()
				return l
			}(),
		},
		{
			name: "equal",
			expected: func() pprofile.LabelSlice {
				l := pprofile.NewLabelSlice()
				i1 := l.AppendEmpty()
				i1.SetKey(1)
				i1.SetNum(3)
				i2 := l.AppendEmpty()
				i2.SetKey(2)
				i2.SetNum(4)
				return l
			}(),
			actual: func() pprofile.LabelSlice {
				l := pprofile.NewLabelSlice()
				i1 := l.AppendEmpty()
				i1.SetKey(1)
				i1.SetNum(3)
				i2 := l.AppendEmpty()
				i2.SetKey(2)
				i2.SetNum(4)
				return l
			}(),
		},
		{
			name: "equal wrong order",
			expected: func() pprofile.LabelSlice {
				l := pprofile.NewLabelSlice()
				i1 := l.AppendEmpty()
				i1.SetKey(1)
				i1.SetNum(3)
				i2 := l.AppendEmpty()
				i2.SetKey(2)
				i2.SetNum(4)
				return l
			}(),
			actual: func() pprofile.LabelSlice {
				l := pprofile.NewLabelSlice()
				i2 := l.AppendEmpty()
				i2.SetKey(2)
				i2.SetNum(4)
				i1 := l.AppendEmpty()
				i1.SetKey(1)
				i1.SetNum(3)
				return l
			}(),
			err: multierr.Combine(
				errors.New(`labels are out of order: label "key: 1" expected at index 0, found at index 1`),
				errors.New(`labels are out of order: label "key: 2" expected at index 1, found at index 0`),
			),
		},
		{
			name: "wrong length",
			expected: func() pprofile.LabelSlice {
				l := pprofile.NewLabelSlice()
				i1 := l.AppendEmpty()
				i1.SetKey(1)
				i1.SetNum(3)
				return l
			}(),
			actual: func() pprofile.LabelSlice {
				l := pprofile.NewLabelSlice()
				i1 := l.AppendEmpty()
				i1.SetKey(1)
				i1.SetNum(3)
				i2 := l.AppendEmpty()
				i2.SetKey(2)
				i2.SetNum(4)
				return l
			}(),
			err: multierr.Combine(
				errors.New(`number of labels doesn't match expected: 1, actual: 2`),
			),
		},
		{
			name: "not equal - does not match expected",
			expected: func() pprofile.LabelSlice {
				l := pprofile.NewLabelSlice()
				i1 := l.AppendEmpty()
				i1.SetKey(1)
				i1.SetNum(3)
				i2 := l.AppendEmpty()
				i2.SetKey(2)
				i2.SetNum(4)
				return l
			}(),
			actual: func() pprofile.LabelSlice {
				l := pprofile.NewLabelSlice()
				i1 := l.AppendEmpty()
				i1.SetKey(1)
				i1.SetNum(3)
				i2 := l.AppendEmpty()
				i2.SetKey(2)
				i2.SetNum(5)
				return l
			}(),
			err: multierr.Combine(
				errors.New(`label with "key: 2" does not match expected`),
			),
		},
		{
			name: "not equal - missing",
			expected: func() pprofile.LabelSlice {
				l := pprofile.NewLabelSlice()
				i1 := l.AppendEmpty()
				i1.SetKey(1)
				i1.SetNum(3)
				i2 := l.AppendEmpty()
				i2.SetKey(2)
				i2.SetNum(4)
				return l
			}(),
			actual: func() pprofile.LabelSlice {
				l := pprofile.NewLabelSlice()
				i1 := l.AppendEmpty()
				i1.SetKey(1)
				i1.SetNum(3)
				i2 := l.AppendEmpty()
				i2.SetKey(3)
				i2.SetNum(6)
				return l
			}(),
			err: multierr.Combine(
				errors.New(`missing expected label "key: 2"`),
				errors.New(`unexpected label "key: 3"`),
			),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.err, CompareProfileLabelSlice(test.expected, test.actual))
		})
	}
}

func TestCompareProfileMappingSlice(t *testing.T) {
	tests := []struct {
		name     string
		expected pprofile.MappingSlice
		actual   pprofile.MappingSlice
		err      error
	}{
		{
			name: "empty",
			expected: func() pprofile.MappingSlice {
				l := pprofile.NewMappingSlice()
				return l
			}(),
			actual: func() pprofile.MappingSlice {
				l := pprofile.NewMappingSlice()
				return l
			}(),
		},
		{
			name: "equal",
			expected: func() pprofile.MappingSlice {
				l := pprofile.NewMappingSlice()
				i1 := l.AppendEmpty()
				i1.SetID(1)
				i1.Attributes().Append(1)
				i1.SetFilename(1)
				i2 := l.AppendEmpty()
				i2.SetID(2)
				i2.Attributes().Append(1)
				i2.SetFilename(2)
				return l
			}(),
			actual: func() pprofile.MappingSlice {
				l := pprofile.NewMappingSlice()
				i1 := l.AppendEmpty()
				i1.SetID(1)
				i1.Attributes().Append(1)
				i1.SetFilename(1)
				i2 := l.AppendEmpty()
				i2.SetID(2)
				i2.Attributes().Append(1)
				i2.SetFilename(2)
				return l
			}(),
		},
		{
			name: "equal wrong order",
			expected: func() pprofile.MappingSlice {
				l := pprofile.NewMappingSlice()
				i1 := l.AppendEmpty()
				i1.SetID(1)
				i1.Attributes().Append(1)
				i1.SetFilename(1)
				i2 := l.AppendEmpty()
				i2.SetID(2)
				i2.Attributes().Append(1)
				i2.SetFilename(2)
				return l
			}(),
			actual: func() pprofile.MappingSlice {
				l := pprofile.NewMappingSlice()
				i2 := l.AppendEmpty()
				i2.SetID(2)
				i2.Attributes().Append(1)
				i2.SetFilename(2)
				i1 := l.AppendEmpty()
				i1.SetID(1)
				i1.Attributes().Append(1)
				i1.SetFilename(1)
				return l
			}(),
			err: multierr.Combine(
				errors.New(`mappings are out of order: mapping "attributes: [1], id: 1" expected at index 0, found at index 1`),
				errors.New(`mappings are out of order: mapping "attributes: [1], id: 2" expected at index 1, found at index 0`),
			),
		},
		{
			name: "wrong length",
			expected: func() pprofile.MappingSlice {
				l := pprofile.NewMappingSlice()
				i2 := l.AppendEmpty()
				i2.SetID(2)
				i2.SetFilename(2)
				return l
			}(),
			actual: func() pprofile.MappingSlice {
				l := pprofile.NewMappingSlice()
				i1 := l.AppendEmpty()
				i1.SetID(1)
				i1.Attributes().Append(1)
				i1.SetFilename(1)
				i2 := l.AppendEmpty()
				i2.SetID(2)
				i2.Attributes().Append(1)
				i2.SetFilename(2)
				return l
			}(),
			err: multierr.Combine(
				errors.New(`number of mappings doesn't match expected: 1, actual: 2`),
			),
		},
		{
			name: "not equal - does not match expected",
			expected: func() pprofile.MappingSlice {
				l := pprofile.NewMappingSlice()
				i1 := l.AppendEmpty()
				i1.SetID(1)
				i1.Attributes().Append(1)
				i1.SetFilename(1)
				i2 := l.AppendEmpty()
				i2.SetID(2)
				i2.Attributes().Append(1)
				i2.SetFilename(2)
				return l
			}(),
			actual: func() pprofile.MappingSlice {
				l := pprofile.NewMappingSlice()
				i1 := l.AppendEmpty()
				i1.SetID(1)
				i1.Attributes().Append(1)
				i1.SetFilename(1)
				i2 := l.AppendEmpty()
				i2.SetID(2)
				i2.Attributes().Append(1)
				i2.SetFilename(3)
				return l
			}(),
			err: multierr.Combine(
				errors.New(`mapping with "attributes: [1], id: 2", does not match expected`),
			),
		},
		{
			name: "not equal - missing",
			expected: func() pprofile.MappingSlice {
				l := pprofile.NewMappingSlice()
				i1 := l.AppendEmpty()
				i1.SetID(1)
				i1.Attributes().Append(1)
				i1.SetFilename(1)
				i2 := l.AppendEmpty()
				i2.SetID(2)
				i2.Attributes().Append(1)
				i2.SetFilename(2)
				return l
			}(),
			actual: func() pprofile.MappingSlice {
				l := pprofile.NewMappingSlice()
				i1 := l.AppendEmpty()
				i1.SetID(1)
				i1.Attributes().Append(1)
				i1.SetFilename(1)
				i2 := l.AppendEmpty()
				i2.SetID(3)
				i2.Attributes().Append(1, 2)
				i2.SetFilename(2)
				return l
			}(),
			err: multierr.Combine(
				errors.New(`missing expected mapping "attributes: [1], id: 2"`),
				errors.New(`unexpected profile mapping "attributes: [1 2], id: 3"`),
			),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.err, CompareProfileMappingSlice(test.expected, test.actual))
		})
	}
}

func TestCompareProfileFunctionSlice(t *testing.T) {
	tests := []struct {
		name     string
		expected pprofile.FunctionSlice
		actual   pprofile.FunctionSlice
		err      error
	}{
		{
			name: "empty",
			expected: func() pprofile.FunctionSlice {
				l := pprofile.NewFunctionSlice()
				return l
			}(),
			actual: func() pprofile.FunctionSlice {
				l := pprofile.NewFunctionSlice()
				return l
			}(),
		},
		{
			name: "equal",
			expected: func() pprofile.FunctionSlice {
				l := pprofile.NewFunctionSlice()
				i1 := l.AppendEmpty()
				i1.SetID(1)
				i1.SetName(1)
				i1.SetFilename(1)
				i2 := l.AppendEmpty()
				i2.SetID(2)
				i2.SetName(2)
				i2.SetFilename(2)
				return l
			}(),
			actual: func() pprofile.FunctionSlice {
				l := pprofile.NewFunctionSlice()
				i1 := l.AppendEmpty()
				i1.SetID(1)
				i1.SetName(1)
				i1.SetFilename(1)
				i2 := l.AppendEmpty()
				i2.SetID(2)
				i2.SetName(2)
				i2.SetFilename(2)
				return l
			}(),
		},
		{
			name: "equal wrong order",
			expected: func() pprofile.FunctionSlice {
				l := pprofile.NewFunctionSlice()
				i1 := l.AppendEmpty()
				i1.SetID(1)
				i1.SetName(1)
				i1.SetFilename(1)
				i2 := l.AppendEmpty()
				i2.SetID(2)
				i2.SetName(2)
				i2.SetFilename(2)
				return l
			}(),
			actual: func() pprofile.FunctionSlice {
				l := pprofile.NewFunctionSlice()
				i2 := l.AppendEmpty()
				i2.SetID(2)
				i2.SetName(2)
				i2.SetFilename(2)
				i1 := l.AppendEmpty()
				i1.SetID(1)
				i1.SetName(1)
				i1.SetFilename(1)
				return l
			}(),
			err: multierr.Combine(
				errors.New(`functions are out of order: function "name: 1" expected at index 0, found at index 1`),
				errors.New(`functions are out of order: function "name: 2" expected at index 1, found at index 0`),
			),
		},
		{
			name: "wrong length",
			expected: func() pprofile.FunctionSlice {
				l := pprofile.NewFunctionSlice()
				i2 := l.AppendEmpty()
				i2.SetID(2)
				i2.SetName(2)
				i2.SetFilename(2)
				return l
			}(),
			actual: func() pprofile.FunctionSlice {
				l := pprofile.NewFunctionSlice()
				i1 := l.AppendEmpty()
				i1.SetID(1)
				i1.SetName(1)
				i1.SetFilename(1)
				i2 := l.AppendEmpty()
				i2.SetID(2)
				i2.SetName(2)
				i2.SetFilename(2)
				return l
			}(),
			err: multierr.Combine(
				errors.New(`number of functions doesn't match expected: 1, actual: 2`),
			),
		},
		{
			name: "not equal - does not match expected",
			expected: func() pprofile.FunctionSlice {
				l := pprofile.NewFunctionSlice()
				i1 := l.AppendEmpty()
				i1.SetID(1)
				i1.SetName(1)
				i1.SetFilename(1)
				i2 := l.AppendEmpty()
				i2.SetID(2)
				i2.SetName(2)
				i2.SetFilename(2)
				return l
			}(),
			actual: func() pprofile.FunctionSlice {
				l := pprofile.NewFunctionSlice()
				i1 := l.AppendEmpty()
				i1.SetID(1)
				i1.SetName(1)
				i1.SetFilename(1)
				i2 := l.AppendEmpty()
				i2.SetID(2)
				i2.SetName(2)
				i2.SetFilename(3)
				return l
			}(),
			err: multierr.Combine(
				errors.New(`function with "name: 2" does not match expected`),
			),
		},
		{
			name: "not equal - missing",
			expected: func() pprofile.FunctionSlice {
				l := pprofile.NewFunctionSlice()
				i1 := l.AppendEmpty()
				i1.SetID(1)
				i1.SetName(1)
				i1.SetFilename(1)
				i2 := l.AppendEmpty()
				i2.SetID(2)
				i2.SetName(2)
				i2.SetFilename(2)
				return l
			}(),
			actual: func() pprofile.FunctionSlice {
				l := pprofile.NewFunctionSlice()
				i1 := l.AppendEmpty()
				i1.SetID(1)
				i1.SetName(1)
				i1.SetFilename(1)
				i2 := l.AppendEmpty()
				i2.SetID(3)
				i2.SetName(3)
				i2.SetFilename(3)
				return l
			}(),
			err: multierr.Combine(
				errors.New(`missing expected function "name: 2"`),
				errors.New(`unexpected profile function "name: 3"`),
			),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.err, CompareProfileFunctionSlice(test.expected, test.actual))
		})
	}
}

func TestCompareProfileLocationSlice(t *testing.T) {
	tests := []struct {
		name     string
		expected pprofile.LocationSlice
		actual   pprofile.LocationSlice
		err      error
	}{
		{
			name: "empty",
			expected: func() pprofile.LocationSlice {
				l := pprofile.NewLocationSlice()
				return l
			}(),
			actual: func() pprofile.LocationSlice {
				l := pprofile.NewLocationSlice()
				return l
			}(),
		},
		{
			name: "equal",
			expected: func() pprofile.LocationSlice {
				l := pprofile.NewLocationSlice()
				i1 := l.AppendEmpty()
				i1.SetID(1)
				i1.Attributes().Append(1, 2)
				i1.SetMappingIndex(1)
				i2 := l.AppendEmpty()
				i2.SetID(2)
				i2.Attributes().Append(1, 2, 3)
				i2.SetMappingIndex(2)
				return l
			}(),
			actual: func() pprofile.LocationSlice {
				l := pprofile.NewLocationSlice()
				i1 := l.AppendEmpty()
				i1.SetID(1)
				i1.Attributes().Append(1, 2)
				i1.SetMappingIndex(1)
				i2 := l.AppendEmpty()
				i2.SetID(2)
				i2.Attributes().Append(1, 2, 3)
				i2.SetMappingIndex(2)
				return l
			}(),
		},
		{
			name: "equal wrong order",
			expected: func() pprofile.LocationSlice {
				l := pprofile.NewLocationSlice()
				i1 := l.AppendEmpty()
				i1.SetID(1)
				i1.Attributes().Append(1, 2)
				i1.SetMappingIndex(1)
				i2 := l.AppendEmpty()
				i2.SetID(2)
				i2.Attributes().Append(1, 2, 3)
				i2.SetMappingIndex(2)
				return l
			}(),
			actual: func() pprofile.LocationSlice {
				l := pprofile.NewLocationSlice()
				i2 := l.AppendEmpty()
				i2.SetID(2)
				i2.Attributes().Append(1, 2, 3)
				i2.SetMappingIndex(2)
				i1 := l.AppendEmpty()
				i1.SetID(1)
				i1.Attributes().Append(1, 2)
				i1.SetMappingIndex(1)
				return l
			}(),
			err: multierr.Combine(
				errors.New(`locations are out of order: location "attributes: [1 2], id: 1" expected at index 0, found at index 1`),
				errors.New(`locations are out of order: location "attributes: [1 2 3], id: 2" expected at index 1, found at index 0`),
			),
		},
		{
			name: "wrong length",
			expected: func() pprofile.LocationSlice {
				l := pprofile.NewLocationSlice()
				i2 := l.AppendEmpty()
				i2.SetID(2)
				i2.Attributes().Append(1, 2, 3)
				i2.SetMappingIndex(2)
				return l
			}(),
			actual: func() pprofile.LocationSlice {
				l := pprofile.NewLocationSlice()
				i2 := l.AppendEmpty()
				i2.SetID(2)
				i2.Attributes().Append(1, 2, 3)
				i2.SetMappingIndex(2)
				i1 := l.AppendEmpty()
				i1.SetID(1)
				i1.Attributes().Append(1, 2)
				i1.SetMappingIndex(1)
				return l
			}(),
			err: multierr.Combine(
				errors.New(`number of locations doesn't match expected: 1, actual: 2`),
			),
		},
		{
			name: "not equal - does not match expected",
			expected: func() pprofile.LocationSlice {
				l := pprofile.NewLocationSlice()
				i1 := l.AppendEmpty()
				i1.SetID(1)
				i1.Attributes().Append(1, 2)
				i1.SetMappingIndex(1)
				i2 := l.AppendEmpty()
				i2.SetID(2)
				i2.Attributes().Append(1, 2, 3)
				i2.SetMappingIndex(2)
				return l
			}(),
			actual: func() pprofile.LocationSlice {
				l := pprofile.NewLocationSlice()
				i1 := l.AppendEmpty()
				i1.SetID(1)
				i1.Attributes().Append(1, 2)
				i1.SetMappingIndex(1)
				i2 := l.AppendEmpty()
				i2.SetID(2)
				i2.Attributes().Append(1, 2, 3)
				i2.SetMappingIndex(3)
				return l
			}(),
			err: multierr.Combine(
				fmt.Errorf(`location "id: 2": %w`, fmt.Errorf(`expected mappingIndex '2', got '3'`)),
			),
		},
		{
			name: "not equal - missing",
			expected: func() pprofile.LocationSlice {
				l := pprofile.NewLocationSlice()
				i1 := l.AppendEmpty()
				i1.SetID(1)
				i1.Attributes().Append(1, 2)
				i1.SetMappingIndex(1)
				i2 := l.AppendEmpty()
				i2.SetID(2)
				i2.Attributes().Append(1, 2, 3)
				i2.SetMappingIndex(2)
				return l
			}(),
			actual: func() pprofile.LocationSlice {
				l := pprofile.NewLocationSlice()
				i1 := l.AppendEmpty()
				i1.SetID(1)
				i1.Attributes().Append(1, 2)
				i1.SetMappingIndex(1)
				i2 := l.AppendEmpty()
				i2.SetID(3)
				i2.Attributes().Append(1, 2, 3, 5)
				i2.SetMappingIndex(2)
				return l
			}(),
			err: multierr.Combine(
				errors.New(`missing expected location "attributes: [1 2 3], id: 2"`),
				errors.New(`unexpected location "attributes: [1 2 3 5], id: 3"`),
			),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.err, CompareProfileLocationSlice(test.expected, test.actual))
		})
	}
}

func TestCompareProfileLocation(t *testing.T) {
	tests := []struct {
		name     string
		expected pprofile.Location
		actual   pprofile.Location
		err      error
	}{
		{
			name: "empty",
			expected: func() pprofile.Location {
				l := pprofile.NewLocation()
				return l
			}(),
			actual: func() pprofile.Location {
				l := pprofile.NewLocation()
				return l
			}(),
		},
		{
			name: "equal",
			expected: func() pprofile.Location {
				l := pprofile.NewLocation()
				l.SetID(1)
				l.SetAddress(2)
				l.SetIsFolded(true)
				l.SetMappingIndex(4)
				l.SetTypeIndex(2)
				l.Attributes().Append(1, 2, 3)
				l.Line().AppendEmpty().Line()
				return l
			}(),
			actual: func() pprofile.Location {
				l := pprofile.NewLocation()
				l.SetID(1)
				l.SetAddress(2)
				l.SetIsFolded(true)
				l.SetMappingIndex(4)
				l.SetTypeIndex(2)
				l.Attributes().Append(1, 2, 3)
				l.Line().AppendEmpty()
				return l
			}(),
		},
		{
			name: "not equal",
			expected: func() pprofile.Location {
				l := pprofile.NewLocation()
				l.SetID(1)
				l.SetAddress(3)
				l.SetIsFolded(false)
				l.SetMappingIndex(2)
				l.SetTypeIndex(3)
				l.Attributes().Append(1, 2, 3, 4)
				l.Line().AppendEmpty().SetFunctionIndex(3)
				return l
			}(),
			actual: func() pprofile.Location {
				l := pprofile.NewLocation()
				l.SetID(1)
				l.SetAddress(2)
				l.SetIsFolded(true)
				l.SetMappingIndex(4)
				l.SetTypeIndex(2)
				l.Attributes().Append(1, 2, 3)
				l.Line().AppendEmpty().Line()
				return l
			}(),
			err: multierr.Combine(
				errors.New(`expected mappingIndex '2', got '4'`),
				errors.New(`expected address '3', got '2'`),
				errors.New(`expected isFolded 'false', got 'true'`),
				errors.New(`expected typeIndex '3', got '2'`),
				errors.New(`expected attributes '[1 2 3 4]', got '[1 2 3]'`),
				fmt.Errorf(`line od location with "id: 1": %w`, fmt.Errorf(`missing expected line "functionIndex: 3"`)),
				fmt.Errorf(`line od location with "id: 1": %w`, fmt.Errorf(`unexpected profile line "functionIndex: 0"`)),
			),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.err, CompareProfileLocation(test.expected, test.actual))
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
			require.Equal(t, test.err, CompareProfileLineSlice(test.expected, test.actual))
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
			require.Equal(t, test.err, CompareProfileAttributeUnitSlice(test.expected, test.actual))
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
			require.Equal(t, test.err, CompareProfileLinkSlice(test.expected, test.actual))
		})
	}
}
