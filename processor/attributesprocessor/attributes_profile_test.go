// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package attributesprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/collector/processor/xprocessor"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/attraction"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/coreinternal/testdata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterconfig"
	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pprofiletest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor/internal/metadata"
)

// Common structure for all the Tests
type profileTestCase struct {
	name               string
	inputAttributes    map[string]any
	expectedAttributes map[string]any
}

// runIndividualProfileTestCase is the common logic of passing profile data through a configured attributes processor.
func runIndividualProfileTestCase(t *testing.T, tt profileTestCase, tp xprocessor.Profiles) {
	t.Run(tt.name, func(t *testing.T) {
		ld := generateProfileData(tt.name, tt.inputAttributes)
		assert.NoError(t, tp.ConsumeProfiles(context.Background(), ld))
		assert.NoError(t, pprofiletest.CompareProfiles(generateProfileData(tt.name, tt.expectedAttributes), ld))
	})
}

func generateProfileData(resourceName string, attrs map[string]any) pprofile.Profiles {
	profileAttrs := make([]pprofiletest.Attribute, 0, len(attrs))
	for k, v := range attrs {
		profileAttrs = append(profileAttrs, pprofiletest.Attribute{Key: k, Value: v})
	}
	return pprofiletest.Profiles{
		ResourceProfiles: []pprofiletest.ResourceProfile{
			{
				Resource: pprofiletest.Resource{
					Attributes: []pprofiletest.Attribute{{Key: "name", Value: resourceName}},
				},
				ScopeProfiles: []pprofiletest.ScopeProfile{
					{
						Profile: []pprofiletest.Profile{
							{
								Attributes: profileAttrs,
								ProfileID:  pprofile.NewProfileIDEmpty(),
							},
						},
					},
				},
			},
		},
	}.Transform()
}

func TestProfileProcessor_NilEmptyData(t *testing.T) {
	type nilEmptyTestCase struct {
		name   string
		input  pprofile.Profiles
		output pprofile.Profiles
	}
	testCases := []nilEmptyTestCase{
		{
			name:   "empty",
			input:  pprofile.NewProfiles(),
			output: pprofile.NewProfiles(),
		},
		{
			name:   "one-empty-resource-profiles",
			input:  testdata.GenerateProfilesOneEmptyResourceProfiles(),
			output: testdata.GenerateProfilesOneEmptyResourceProfiles(),
		},
		{
			name:   "no-libraries",
			input:  testdata.GenerateProfilesOneEmptyResourceProfiles(),
			output: testdata.GenerateProfilesOneEmptyResourceProfiles(),
		},
	}
	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Settings.Actions = []attraction.ActionKeyValue{
		{Key: "attribute1", Action: attraction.INSERT, Value: 123},
		{Key: "attribute1", Action: attraction.DELETE},
	}

	tp, err := factory.(xprocessor.Factory).CreateProfiles(
		context.Background(), processortest.NewNopSettings(metadata.Type), oCfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, tp)
	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			assert.NoError(t, tp.ConsumeProfiles(context.Background(), tt.input))
			assert.EqualValues(t, tt.output, tt.input)
		})
	}
}

func TestAttributes_FilterProfiles(t *testing.T) {
	testCases := []profileTestCase{
		{
			name:            "apply processor",
			inputAttributes: map[string]any{},
			expectedAttributes: map[string]any{
				"attribute1": 123,
			},
		},
		{
			name: "apply processor with different value for exclude property",
			inputAttributes: map[string]any{
				"NoModification": false,
			},
			expectedAttributes: map[string]any{
				"attribute1":     123,
				"NoModification": false,
			},
		},
		{
			name:               "incorrect name for include property",
			inputAttributes:    map[string]any{},
			expectedAttributes: map[string]any{},
		},
		{
			name: "attribute match for exclude property",
			inputAttributes: map[string]any{
				"NoModification": true,
			},
			expectedAttributes: map[string]any{
				"NoModification": true,
			},
		},
	}

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Actions = []attraction.ActionKeyValue{
		{Key: "attribute1", Action: attraction.INSERT, Value: 123},
	}
	oCfg.Include = &filterconfig.MatchProperties{
		Resources: []filterconfig.Attribute{{Key: "name", Value: "^[^i].*"}},
		// Libraries: []filterconfig.InstrumentationLibrary{{Name: "^[^i].*"}},
		Config: *createConfig(filterset.Regexp),
	}
	oCfg.Exclude = &filterconfig.MatchProperties{
		Attributes: []filterconfig.Attribute{
			{Key: "NoModification", Value: true},
		},
		Config: *createConfig(filterset.Strict),
	}
	tp, err := factory.(xprocessor.Factory).CreateProfiles(
		context.Background(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, tp)

	for _, tt := range testCases {
		runIndividualProfileTestCase(t, tt, tp)
	}
}

func TestAttributes_FilterProfilesByNameStrict(t *testing.T) {
	testCases := []profileTestCase{
		{
			name:            "apply",
			inputAttributes: map[string]any{},
			expectedAttributes: map[string]any{
				"attribute1": 123,
			},
		},
		{
			name: "apply",
			inputAttributes: map[string]any{
				"NoModification": false,
			},
			expectedAttributes: map[string]any{
				"attribute1":     123,
				"NoModification": false,
			},
		},
		{
			name:               "incorrect_profile_name",
			inputAttributes:    map[string]any{},
			expectedAttributes: map[string]any{},
		},
		{
			name:               "dont_apply",
			inputAttributes:    map[string]any{},
			expectedAttributes: map[string]any{},
		},
		{
			name: "incorrect_profile_name_with_attr",
			inputAttributes: map[string]any{
				"NoModification": true,
			},
			expectedAttributes: map[string]any{
				"NoModification": true,
			},
		},
	}

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Actions = []attraction.ActionKeyValue{
		{Key: "attribute1", Action: attraction.INSERT, Value: 123},
	}
	oCfg.Include = &filterconfig.MatchProperties{
		Resources: []filterconfig.Attribute{{Key: "name", Value: "apply"}},
		Config:    *createConfig(filterset.Strict),
	}
	oCfg.Exclude = &filterconfig.MatchProperties{
		Resources: []filterconfig.Attribute{{Key: "name", Value: "dont_apply"}},
		Config:    *createConfig(filterset.Strict),
	}
	tp, err := factory.(xprocessor.Factory).CreateProfiles(
		context.Background(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, tp)

	for _, tt := range testCases {
		runIndividualProfileTestCase(t, tt, tp)
	}
}

func TestAttributes_FilterProfilesByNameRegexp(t *testing.T) {
	testCases := []profileTestCase{
		{
			name:            "apply_to_profile_with_no_attrs",
			inputAttributes: map[string]any{},
			expectedAttributes: map[string]any{
				"attribute1": 123,
			},
		},
		{
			name: "apply_to_profile_with_attr",
			inputAttributes: map[string]any{
				"NoModification": false,
			},
			expectedAttributes: map[string]any{
				"attribute1":     123,
				"NoModification": false,
			},
		},
		{
			name:               "incorrect_profile_name",
			inputAttributes:    map[string]any{},
			expectedAttributes: map[string]any{},
		},
		{
			name:               "apply_dont_apply",
			inputAttributes:    map[string]any{},
			expectedAttributes: map[string]any{},
		},
		{
			name: "incorrect_profile_name_with_attr",
			inputAttributes: map[string]any{
				"NoModification": true,
			},
			expectedAttributes: map[string]any{
				"NoModification": true,
			},
		},
	}

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Actions = []attraction.ActionKeyValue{
		{Key: "attribute1", Action: attraction.INSERT, Value: 123},
	}
	oCfg.Include = &filterconfig.MatchProperties{
		Resources: []filterconfig.Attribute{{Key: "name", Value: "^apply.*"}},
		Config:    *createConfig(filterset.Regexp),
	}
	oCfg.Exclude = &filterconfig.MatchProperties{
		Resources: []filterconfig.Attribute{{Key: "name", Value: ".*dont_apply$"}},
		Config:    *createConfig(filterset.Regexp),
	}
	tp, err := factory.(xprocessor.Factory).CreateProfiles(
		context.Background(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, tp)

	for _, tt := range testCases {
		runIndividualProfileTestCase(t, tt, tp)
	}
}

func TestProfileAttributes_Hash(t *testing.T) {
	testCases := []profileTestCase{
		{
			name: "String",
			inputAttributes: map[string]any{
				"user.email": "john.doe@example.com",
			},
			expectedAttributes: map[string]any{
				"user.email": "836f82db99121b3481011f16b49dfa5fbc714a0d1b1b9f784a1ebbbf5b39577f",
			},
		},
		{
			name: "Int",
			inputAttributes: map[string]any{
				"user.id": 10,
			},
			expectedAttributes: map[string]any{
				"user.id": "a111f275cc2e7588000001d300a31e76336d15b9d314cd1a1d8f3d3556975eed",
			},
		},
		{
			name: "Double",
			inputAttributes: map[string]any{
				"user.balance": 99.1,
			},
			expectedAttributes: map[string]any{
				"user.balance": "05fabd78b01be9692863cb0985f600c99da82979af18db5c55173c2a30adb924",
			},
		},
		{
			name: "Bool",
			inputAttributes: map[string]any{
				"user.authenticated": true,
			},
			expectedAttributes: map[string]any{
				"user.authenticated": "4bf5122f344554c53bde2ebb8cd2b7e3d1600ad631c385a5d7cce23c7785459a",
			},
		},
	}

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Actions = []attraction.ActionKeyValue{
		{Key: "user.email", Action: attraction.HASH},
		{Key: "user.id", Action: attraction.HASH},
		{Key: "user.balance", Action: attraction.HASH},
		{Key: "user.authenticated", Action: attraction.HASH},
	}

	tp, err := factory.(xprocessor.Factory).CreateProfiles(
		context.Background(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, tp)

	for _, tt := range testCases {
		runIndividualProfileTestCase(t, tt, tp)
	}
}

func TestProfileAttributes_Convert(t *testing.T) {
	testCases := []profileTestCase{
		{
			name: "int to int",
			inputAttributes: map[string]any{
				"to.int": 1,
			},
			expectedAttributes: map[string]any{
				"to.int": 1,
			},
		},
		{
			name: "false to int",
			inputAttributes: map[string]any{
				"to.int": false,
			},
			expectedAttributes: map[string]any{
				"to.int": 0,
			},
		},
		{
			name: "String to int (good)",
			inputAttributes: map[string]any{
				"to.int": "123",
			},
			expectedAttributes: map[string]any{
				"to.int": 123,
			},
		},
		{
			name: "String to int (bad)",
			inputAttributes: map[string]any{
				"to.int": "int-10",
			},
			expectedAttributes: map[string]any{
				"to.int": "int-10",
			},
		},
		{
			name: "String to double",
			inputAttributes: map[string]any{
				"to.double": "123.6",
			},
			expectedAttributes: map[string]any{
				"to.double": 123.6,
			},
		},
		{
			name: "Double to string",
			inputAttributes: map[string]any{
				"to.string": 99.1,
			},
			expectedAttributes: map[string]any{
				"to.string": "99.1",
			},
		},
	}

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Actions = []attraction.ActionKeyValue{
		{Key: "to.int", Action: attraction.CONVERT, ConvertedType: "int"},
		{Key: "to.double", Action: attraction.CONVERT, ConvertedType: "double"},
		{Key: "to.string", Action: attraction.CONVERT, ConvertedType: "string"},
	}

	tp, err := factory.(xprocessor.Factory).CreateProfiles(
		context.Background(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(t, err)
	require.NotNil(t, tp)

	for _, tt := range testCases {
		runIndividualProfileTestCase(t, tt, tp)
	}
}

func BenchmarkAttributes_FilterProfilesByName(b *testing.B) {
	testCases := []profileTestCase{
		{
			name:            "apply_to_profile_with_no_attrs",
			inputAttributes: map[string]any{},
			expectedAttributes: map[string]any{
				"attribute1": 123,
			},
		},
		{
			name: "apply_to_profile_with_attr",
			inputAttributes: map[string]any{
				"NoModification": false,
			},
			expectedAttributes: map[string]any{
				"attribute1":     123,
				"NoModification": false,
			},
		},
		{
			name:               "dont_apply",
			inputAttributes:    map[string]any{},
			expectedAttributes: map[string]any{},
		},
	}

	factory := NewFactory()
	cfg := factory.CreateDefaultConfig()
	oCfg := cfg.(*Config)
	oCfg.Actions = []attraction.ActionKeyValue{
		{Key: "attribute1", Action: attraction.INSERT, Value: 123},
	}
	oCfg.Include = &filterconfig.MatchProperties{
		Config:    *createConfig(filterset.Regexp),
		Resources: []filterconfig.Attribute{{Key: "name", Value: "^apply.*"}},
	}
	tp, err := factory.(xprocessor.Factory).CreateProfiles(
		context.Background(), processortest.NewNopSettings(metadata.Type), cfg, consumertest.NewNop())
	require.NoError(b, err)
	require.NotNil(b, tp)

	for _, tt := range testCases {
		td := generateProfileData(tt.name, tt.inputAttributes)

		b.Run(tt.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				assert.NoError(b, tp.ConsumeProfiles(context.Background(), td))
			}
		})

		require.NoError(b, pprofiletest.CompareProfiles(generateProfileData(tt.name, tt.expectedAttributes), td))
	}
}
