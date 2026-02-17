// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterprocessor

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/collector/processor/xprocessor"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/ottlprofile"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pprofiletest"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/condition"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/metadatatest"
)

var profileID = [16]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}

func TestNilResourceProfiles(t *testing.T) {
	profiles := pprofile.NewProfiles()
	rps := profiles.ResourceProfiles()
	rps.AppendEmpty()
	requireNotPanicsProfiles(t, profiles)
}

func TestNilScopeProfiles(t *testing.T) {
	profiles := pprofile.NewProfiles()
	rps := profiles.ResourceProfiles()
	rl := rps.AppendEmpty()
	ills := rl.ScopeProfiles()
	ills.AppendEmpty()
	requireNotPanicsProfiles(t, profiles)
}

func TestNilProfile(t *testing.T) {
	profiles := pprofile.NewProfiles()
	rps := profiles.ResourceProfiles()
	rl := rps.AppendEmpty()
	sps := rl.ScopeProfiles()
	sp := sps.AppendEmpty()
	ps := sp.Profiles()
	ps.AppendEmpty()
	requireNotPanicsProfiles(t, profiles)
}

func requireNotPanicsProfiles(t *testing.T, profiles pprofile.Profiles) {
	factory := NewFactory().(xprocessor.Factory)
	cfg := factory.CreateDefaultConfig()
	pcfg := cfg.(*Config)
	pcfg.Profiles = ProfileFilters{}
	ctx := t.Context()
	proc, _ := factory.CreateProfiles(
		ctx,
		processortest.NewNopSettings(metadata.Type),
		cfg,
		consumertest.NewNop(),
	)
	require.NotPanics(t, func() {
		_ = proc.ConsumeProfiles(ctx, profiles)
	})
}

func TestFilterProfileProcessorWithOTTL(t *testing.T) {
	tests := []struct {
		name              string
		conditions        ProfileFilters
		contextConditions []condition.ContextConditions
		filterEverything  bool
		want              func(pprofile.Profiles)
		errorMode         ottl.ErrorMode
	}{
		{
			name: "drop resource",
			conditions: ProfileFilters{
				ResourceConditions: []string{
					`attributes["host.name"] == "localhost"`,
				},
			},
			filterEverything: true,
			errorMode:        ottl.IgnoreError,
		},
		{
			name: "drop profiles",
			conditions: ProfileFilters{
				ProfileConditions: []string{
					`original_payload_format == "legacy"`,
				},
			},
			want: func(ld pprofile.Profiles) {
				ld.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().RemoveIf(func(profile pprofile.Profile) bool {
					return profile.OriginalPayloadFormat() == "legacy"
				})
				ld.ResourceProfiles().At(0).ScopeProfiles().At(1).Profiles().RemoveIf(func(profile pprofile.Profile) bool {
					return profile.OriginalPayloadFormat() == "legacy"
				})
			},
			errorMode: ottl.IgnoreError,
		},
		{
			name: "drop everything by dropping all profiles",
			conditions: ProfileFilters{
				ProfileConditions: []string{
					`IsMatch(original_payload_format, ".*legacy")`,
				},
			},
			filterEverything: true,
			errorMode:        ottl.IgnoreError,
		},
		{
			name: "multiple conditions",
			conditions: ProfileFilters{
				ProfileConditions: []string{
					`IsMatch(original_payload_format, "wrong name")`,
					`IsMatch(original_payload_format, ".*legacy")`,
				},
			},
			filterEverything: true,
			errorMode:        ottl.IgnoreError,
		},
		{
			name: "with error conditions",
			conditions: ProfileFilters{
				ProfileConditions: []string{
					`Substring("", 0, 100) == "test"`,
				},
			},
			want:      func(_ pprofile.Profiles) {},
			errorMode: ottl.IgnoreError,
		},
		{
			name: "with context conditions",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`IsMatch(profile.original_payload_format, ".*legacy")`}},
			},
			filterEverything: true,
			errorMode:        ottl.IgnoreError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Profiles: tt.conditions, ProfileConditions: tt.contextConditions, profileFunctions: defaultProfileFunctionsMap()}
			processor, err := newFilterProfilesProcessor(processortest.NewNopSettings(metadata.Type), cfg)
			assert.NoError(t, err)

			got, err := processor.processProfiles(t.Context(), constructProfiles())

			if tt.filterEverything {
				assert.Equal(t, processorhelper.ErrSkipProcessingData, err)
			} else {
				exTd := constructProfiles()
				tt.want(exTd)
				assert.Equal(t, exTd, got)
			}
		})
	}
}

func TestFilterProfileProcessorTelemetry(t *testing.T) {
	tel := componenttest.NewTelemetry()
	t.Cleanup(func() { require.NoError(t, tel.Shutdown(context.Background())) }) //nolint:usetesting
	processor, err := newFilterProfilesProcessor(metadatatest.NewSettings(tel), &Config{
		Profiles:         ProfileFilters{ProfileConditions: []string{`original_payload_format == "legacy"`}},
		profileFunctions: defaultProfileFunctionsMap(),
	})
	assert.NoError(t, err)

	_, err = processor.processProfiles(t.Context(), constructProfiles2())
	assert.NoError(t, err)

	metadatatest.AssertEqualProcessorFilterProfilesFiltered(t, tel, []metricdata.DataPoint[int64]{
		{
			Value:      2,
			Attributes: attribute.NewSet(attribute.String("filter", "filter")),
		},
	}, metricdatatest.IgnoreTimestamp())
}

func Test_ProcessProfiles_DefinedContext(t *testing.T) {
	tests := []struct {
		name              string
		contextConditions []condition.ContextConditions
		filterEverything  bool
		want              func(pd pprofile.Profiles)
	}{
		{
			name: "resource: drop by schema_url",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`schema_url == "test_schema_url"`}, Context: "resource"},
			},
			want: func(_ pprofile.Profiles) {},
		},
		{
			name: "resource: drop by attribute",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`attributes["host.name"] == "localhost"`}, Context: "resource"},
			},
			filterEverything: true,
		},
		{
			name: "scope: drop by name",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`name == "scope1"`}, Context: "scope"},
			},
			want: func(pd pprofile.Profiles) {
				pd.ResourceProfiles().RemoveIf(func(rp pprofile.ResourceProfiles) bool {
					rp.ScopeProfiles().RemoveIf(func(sp pprofile.ScopeProfiles) bool {
						return sp.Scope().Name() == "scope1"
					})
					return rp.ScopeProfiles().Len() == 0
				})
			},
		},
		{
			name: "scope: drop by attribute",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`attributes["lib"] == "awesomelib"`}, Context: "scope"},
			},
			want: func(_ pprofile.Profiles) {},
		},
		{
			name: "profile: drop by attributes",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`IsMatch(attributes["total.string"], ".*")`}, Context: "profile"},
			},
			want: func(pd pprofile.Profiles) {
				dic := pd.Dictionary()
				pd.ResourceProfiles().RemoveIf(func(rp pprofile.ResourceProfiles) bool {
					rp.ScopeProfiles().RemoveIf(func(sp pprofile.ScopeProfiles) bool {
						sp.Profiles().RemoveIf(func(profile pprofile.Profile) bool {
							indices := profile.AttributeIndices().AsRaw()
							for i := range indices {
								if dic.StringTable().At(int(dic.AttributeTable().At(int(indices[i])).KeyStrindex())) == "total.string" {
									return true
								}
							}
							return false
						})
						return sp.Profiles().Len() == 0
					})
					return rp.ScopeProfiles().Len() == 0
				})
			},
		},
		{
			name: "profile: drop by payload format",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`original_payload_format == "legacy"`}, Context: "profile"},
			},
			want: func(pd pprofile.Profiles) {
				pd.ResourceProfiles().RemoveIf(func(rp pprofile.ResourceProfiles) bool {
					rp.ScopeProfiles().RemoveIf(func(sp pprofile.ScopeProfiles) bool {
						sp.Profiles().RemoveIf(func(profile pprofile.Profile) bool {
							return profile.OriginalPayloadFormat() == "legacy"
						})
						return sp.Profiles().Len() == 0
					})
					return rp.ScopeProfiles().Len() == 0
				})
			},
		},
		{
			name: "profile: drop by function",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`IsMatch(original_payload_format, ".*legacy")`}, Context: "profile"},
			},
			filterEverything: true,
		},
		{
			name: "inferring mixed condition",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`original_payload_format == "legacy"`}, Context: "profile"},
				{Conditions: []string{`scope.name == "scope1"`}, Context: "scope"},
			},
			want: func(pd pprofile.Profiles) {
				pd.ResourceProfiles().RemoveIf(func(rp pprofile.ResourceProfiles) bool {
					rp.ScopeProfiles().RemoveIf(func(sp pprofile.ScopeProfiles) bool {
						sp.Profiles().RemoveIf(func(profile pprofile.Profile) bool {
							return profile.OriginalPayloadFormat() == "legacy"
						})
						return sp.Profiles().Len() == 0 || sp.Scope().Name() == "scope1"
					})
					return rp.ScopeProfiles().Len() == 0
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, _ := NewFactory().CreateDefaultConfig().(*Config)
			cfg.ProfileConditions = tt.contextConditions
			processor, err := newFilterProfilesProcessor(processortest.NewNopSettings(metadata.Type), cfg)
			assert.NoError(t, err)

			got, err := processor.processProfiles(t.Context(), constructProfiles())

			if tt.filterEverything {
				assert.Equal(t, processorhelper.ErrSkipProcessingData, err)
			} else {
				assert.NoError(t, err)
				exTd := constructProfiles()
				tt.want(exTd)
				assert.Equal(t, exTd, got)
			}
		})
	}
}

func Test_ProcessProfiles_InferredContext(t *testing.T) {
	tests := []struct {
		name              string
		contextConditions []condition.ContextConditions
		filterEverything  bool
		want              func(pd pprofile.Profiles)
		input             func() pprofile.Profiles
	}{
		{
			name: "resource: drop by schema_url",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`resource.schema_url == "test_schema_url"`}},
			},
			want:  func(_ pprofile.Profiles) {},
			input: constructProfiles,
		},
		{
			name: "resource: drop by attribute",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`resource.attributes["host.name"] == "localhost"`}},
			},
			filterEverything: true,
			input:            constructProfiles,
		},
		{
			name: "scope: drop by name",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`scope.name == "scope1"`}},
			},
			want: func(pd pprofile.Profiles) {
				pd.ResourceProfiles().RemoveIf(func(rp pprofile.ResourceProfiles) bool {
					rp.ScopeProfiles().RemoveIf(func(sp pprofile.ScopeProfiles) bool {
						return sp.Scope().Name() == "scope1"
					})
					return rp.ScopeProfiles().Len() == 0
				})
			},
			input: constructProfiles,
		},
		{
			name: "scope: drop by attribute",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`scope.attributes["lib"] == "awesomelib"`}},
			},
			want:  func(_ pprofile.Profiles) {},
			input: constructProfiles,
		},
		{
			name: "profile: drop by attributes",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`IsMatch(profile.attributes["total.string"], ".*")`}},
			},
			want: func(pd pprofile.Profiles) {
				dic := pd.Dictionary()
				pd.ResourceProfiles().RemoveIf(func(rp pprofile.ResourceProfiles) bool {
					rp.ScopeProfiles().RemoveIf(func(sp pprofile.ScopeProfiles) bool {
						sp.Profiles().RemoveIf(func(profile pprofile.Profile) bool {
							indices := profile.AttributeIndices().AsRaw()
							for i := range indices {
								if dic.StringTable().At(int(dic.AttributeTable().At(int(indices[i])).KeyStrindex())) == "total.string" {
									return true
								}
							}
							return false
						})
						return sp.Profiles().Len() == 0
					})
					return rp.ScopeProfiles().Len() == 0
				})
			},
			input: constructProfiles,
		},
		{
			name: "profile: drop by payload format",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`profile.original_payload_format == "legacy"`}},
			},
			want: func(pd pprofile.Profiles) {
				pd.ResourceProfiles().RemoveIf(func(rp pprofile.ResourceProfiles) bool {
					rp.ScopeProfiles().RemoveIf(func(sp pprofile.ScopeProfiles) bool {
						sp.Profiles().RemoveIf(func(profile pprofile.Profile) bool {
							return profile.OriginalPayloadFormat() == "legacy"
						})
						return sp.Profiles().Len() == 0
					})
					return rp.ScopeProfiles().Len() == 0
				})
			},
			input: constructProfiles,
		},
		{
			name: "profile: drop by function",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{`IsMatch(profile.original_payload_format, ".*legacy")`}},
			},
			filterEverything: true,
			input:            constructProfiles,
		},
		{
			name: "inferring mixed contexts",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{
					`profile.original_payload_format == "legacy"`,
					`scope.name == "scope1"`,
				}},
			},
			want: func(pd pprofile.Profiles) {
				pd.ResourceProfiles().RemoveIf(func(rp pprofile.ResourceProfiles) bool {
					rp.ScopeProfiles().RemoveIf(func(sp pprofile.ScopeProfiles) bool {
						sp.Profiles().RemoveIf(func(profile pprofile.Profile) bool {
							return profile.OriginalPayloadFormat() == "legacy"
						})
						return sp.Profiles().Len() == 0 || sp.Scope().Name() == "scope1"
					})
					return rp.ScopeProfiles().Len() == 0
				})
			},
			input: constructProfiles,
		},
		{
			name: "zero-record lower-context: resource and profile with no profiles",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{
					`resource.attributes["host.name"] == "localhost"`,
					`profile.original_payload_format == "legacy"`,
				}},
			},
			filterEverything: true,
			input:            constructProfilesWithEmptyProfiles,
		},
		{
			name: "group by context: conditions in same group are respected",
			contextConditions: []condition.ContextConditions{
				{Conditions: []string{
					`scope.name == "scope1"`,
					`scope.name == "scope2"`,
					`profile.original_payload_format == "legacy"`,
					`profile.original_payload_format == "non-legacy"`,
				}},
			},
			filterEverything: true,
			input:            constructProfiles,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, _ := NewFactory().CreateDefaultConfig().(*Config)
			cfg.ProfileConditions = tt.contextConditions
			processor, err := newFilterProfilesProcessor(processortest.NewNopSettings(metadata.Type), cfg)
			assert.NoError(t, err)

			got, err := processor.processProfiles(t.Context(), tt.input())

			if tt.filterEverything {
				assert.Equal(t, processorhelper.ErrSkipProcessingData, err)
			} else {
				assert.NoError(t, err)
				exTd := tt.input()
				tt.want(exTd)
				assert.Equal(t, exTd, got)
			}
		})
	}
}

func Test_ProcessProfiles_ErrorMode(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "resource",
		},
		{
			name: "scope",
		},
		{
			name: "profile",
		},
	}

	for _, errMode := range []ottl.ErrorMode{ottl.PropagateError, ottl.IgnoreError, ottl.SilentError} {
		for _, tt := range tests {
			t.Run(fmt.Sprintf("%s:%s", tt.name, errMode), func(t *testing.T) {
				cfg, _ := NewFactory().CreateDefaultConfig().(*Config)
				cfg.ProfileConditions = []condition.ContextConditions{
					{Conditions: []string{`ParseJSON("1")`}, Context: condition.ContextID(tt.name)},
				}
				cfg.ErrorMode = errMode
				processor, err := newFilterProfilesProcessor(processortest.NewNopSettings(metadata.Type), cfg)
				assert.NoError(t, err)

				_, err = processor.processProfiles(t.Context(), constructProfiles())

				switch errMode {
				case ottl.PropagateError:
					assert.Error(t, err)
				case ottl.IgnoreError, ottl.SilentError:
					assert.NoError(t, err)
				}
			})
		}
	}
}

func Test_ProcessProfiles_ConditionsErrorMode(t *testing.T) {
	tests := []struct {
		name          string
		errorMode     ottl.ErrorMode
		conditions    []condition.ContextConditions
		want          func(pd pprofile.Profiles)
		wantErrorWith string
	}{
		{
			name:      "resource: conditions group with error mode",
			errorMode: ottl.PropagateError,
			conditions: []condition.ContextConditions{
				{Conditions: []string{`resource.attributes["pass"] == ParseJSON("1")`}, ErrorMode: ottl.IgnoreError, Context: condition.ContextID("resource")},
				{Conditions: []string{`not IsMatch(resource.attributes["host.name"], ".*")`}, Context: condition.ContextID("resource")},
			},
			want: func(pd pprofile.Profiles) {
				pd.ResourceProfiles().RemoveIf(func(rp pprofile.ResourceProfiles) bool {
					v, _ := rp.Resource().Attributes().Get("host.name")
					return v.AsString() == ""
				})
			},
		},
		{
			name:      "resource: conditions group error mode does not affect default",
			errorMode: ottl.PropagateError,
			conditions: []condition.ContextConditions{
				{Conditions: []string{`resource.attributes["pass"] == ParseJSON("1")`}, ErrorMode: ottl.IgnoreError, Context: condition.ContextID("resource")},
				{Conditions: []string{`resource.attributes["pass"] == ParseJSON("true")`}, Context: condition.ContextID("resource")},
			},
			wantErrorWith: "could not convert parsed value of type bool to JSON object",
		},
		{
			name:      "resource: conditions group error mode with undefined context takes precedence",
			errorMode: ottl.PropagateError,
			conditions: []condition.ContextConditions{
				{Conditions: []string{`resource.attributes["pass"] == ParseJSON("1")`}, ErrorMode: ottl.IgnoreError},
			},
			want: func(_ pprofile.Profiles) {},
		},
		{
			name:      "scope: conditions group with error mode",
			errorMode: ottl.PropagateError,
			conditions: []condition.ContextConditions{
				{Conditions: []string{`scope.name == ParseJSON("1")`}, ErrorMode: ottl.IgnoreError, Context: condition.ContextID("scope")},
				{Conditions: []string{`scope.name == "scope1"`}, Context: condition.ContextID("scope")},
			},
			want: func(pd pprofile.Profiles) {
				pd.ResourceProfiles().RemoveIf(func(rp pprofile.ResourceProfiles) bool {
					rp.ScopeProfiles().RemoveIf(func(sp pprofile.ScopeProfiles) bool {
						return sp.Scope().Name() == "scope1"
					})
					return rp.ScopeProfiles().Len() == 0
				})
			},
		},
		{
			name:      "scope: conditions group error mode does not affect default",
			errorMode: ottl.PropagateError,
			conditions: []condition.ContextConditions{
				{Conditions: []string{`scope.name == ParseJSON("1")`}, ErrorMode: ottl.IgnoreError, Context: condition.ContextID("scope")},
				{Conditions: []string{`scope.name == ParseJSON("true")`}, Context: condition.ContextID("scope")},
			},
			wantErrorWith: "could not convert parsed value of type bool to JSON object",
		},
		{
			name:      "profile: conditions group with error mode",
			errorMode: ottl.PropagateError,
			conditions: []condition.ContextConditions{
				{Conditions: []string{`profile.original_payload_format == ParseJSON("1")`}, ErrorMode: ottl.IgnoreError, Context: condition.ContextID("profile")},
				{Conditions: []string{`IsMatch(profile.original_payload_format, "non-legacy")`}, Context: condition.ContextID("profile")},
			},
			want: func(pd pprofile.Profiles) {
				pd.ResourceProfiles().RemoveIf(func(rp pprofile.ResourceProfiles) bool {
					rp.ScopeProfiles().RemoveIf(func(sp pprofile.ScopeProfiles) bool {
						sp.Profiles().RemoveIf(func(profile pprofile.Profile) bool {
							return profile.OriginalPayloadFormat() == "non-legacy"
						})
						return sp.Profiles().Len() == 0
					})
					return rp.ScopeProfiles().Len() == 0
				})
			},
		},
		{
			name:      "profile: conditions group error mode does not affect default",
			errorMode: ottl.PropagateError,
			conditions: []condition.ContextConditions{
				{Conditions: []string{`profile.original_payload_format == ParseJSON("1")`}, ErrorMode: ottl.IgnoreError, Context: condition.ContextID("profile")},
				{Conditions: []string{`profile.original_payload_format == ParseJSON("true")`}, Context: condition.ContextID("profile")},
			},
			wantErrorWith: "could not convert parsed value of type bool to JSON object",
		},
		{
			name:      "flat style propagate error",
			errorMode: ottl.PropagateError,
			conditions: []condition.ContextConditions{
				{
					Conditions: []string{
						`resource.attributes["pass"] == ParseJSON("1")`,
						`not IsMatch(resource.attributes["host.name"], ".*")`,
					},
				},
			},
			wantErrorWith: "could not convert parsed value of type float64 to JSON object",
		},
		{
			name:      "flat style ignore error",
			errorMode: ottl.IgnoreError,
			conditions: []condition.ContextConditions{
				{
					Conditions: []string{
						`resource.attributes["pass"] == ParseJSON("1")`,
						`not IsMatch(resource.attributes["host.name"], ".*")`,
					},
				},
			},
			want: func(pd pprofile.Profiles) {
				pd.ResourceProfiles().RemoveIf(func(rp pprofile.ResourceProfiles) bool {
					v, _ := rp.Resource().Attributes().Get("host.name")
					return v.AsString() == ""
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, _ := NewFactory().CreateDefaultConfig().(*Config)
			cfg.ProfileConditions = tt.conditions
			cfg.ErrorMode = tt.errorMode

			processor, err := newFilterProfilesProcessor(processortest.NewNopSettings(metadata.Type), cfg)
			assert.NoError(t, err)

			got, err := processor.processProfiles(t.Context(), constructProfiles())
			if tt.wantErrorWith != "" {
				if err == nil {
					t.Errorf("expected error containing '%s', got: <nil>", tt.wantErrorWith)
				}
				assert.Contains(t, err.Error(), tt.wantErrorWith)
				return
			}

			assert.NoError(t, err)
			exTd := constructProfiles()
			tt.want(exTd)
			assert.Equal(t, exTd, got)
		})
	}
}

func Test_Profiles_NonDefaultFunctions(t *testing.T) {
	type testCase struct {
		name             string
		conditions       []condition.ContextConditions
		wantErrorWith    string
		profileFunctions map[string]ottl.Factory[*ottlprofile.TransformContext]
	}

	tests := []testCase{
		{
			name: "profile funcs : statement with added profile func",
			conditions: []condition.ContextConditions{
				{
					Context:    condition.ContextID("profile"),
					Conditions: []string{`IsMatch(original_payload_format, TestProfileFunc())`},
				},
			},
			profileFunctions: map[string]ottl.Factory[*ottlprofile.TransformContext]{
				"IsMatch":         defaultProfileFunctionsMap()["IsMatch"],
				"TestProfileFunc": NewProfileFuncFactory[*ottlprofile.TransformContext](),
			},
		},
		{
			name: "profile funcs : statement with missing profile func",
			conditions: []condition.ContextConditions{
				{
					Context:    condition.ContextID("profile"),
					Conditions: []string{`IsMatch(original_payload_format, TestProfileFunc())`},
				},
			},
			wantErrorWith:    `undefined function "TestProfileFunc"`,
			profileFunctions: defaultProfileFunctionsMap(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, _ := NewFactory().CreateDefaultConfig().(*Config)
			cfg.ProfileConditions = tt.conditions
			cfg.profileFunctions = tt.profileFunctions

			_, err := newFilterProfilesProcessor(processortest.NewNopSettings(metadata.Type), cfg)

			if tt.wantErrorWith != "" {
				if err == nil {
					t.Errorf("expected error containing '%s', got: <nil>", tt.wantErrorWith)
				}
				assert.Contains(t, err.Error(), tt.wantErrorWith)
				return
			}
			require.NoError(t, err)
		})
	}
}

type ProfileFuncArguments[K any] struct{}

func createProfileFunc[K any](ottl.FunctionContext, ottl.Arguments) (ottl.ExprFunc[K], error) {
	return func(context.Context, K) (any, error) {
		return nil, nil
	}, nil
}

func NewProfileFuncFactory[K any]() ottl.Factory[K] {
	return ottl.NewFactory("TestProfileFunc", &ProfileFuncArguments[K]{}, createProfileFunc[K])
}

func constructProfilesWithEmptyProfiles() pprofile.Profiles {
	pd := pprofile.NewProfiles()
	rp := pd.ResourceProfiles().AppendEmpty()
	rp.Resource().Attributes().PutStr("host.name", "localhost")
	sp := rp.ScopeProfiles().AppendEmpty()
	sp.Scope().SetName("scope1")
	return pd
}

func constructProfiles() pprofile.Profiles {
	return constructTestProfiles().Transform()
}

func constructTestProfiles() pprofiletest.Profiles {
	r := pcommon.NewResource()
	r.Attributes().PutStr("host.name", "localhost")
	s1 := pcommon.NewInstrumentationScope()
	s1.SetName("scope1")
	s2 := pcommon.NewInstrumentationScope()
	s2.SetName("scope2")
	return pprofiletest.Profiles{
		ResourceProfiles: []pprofiletest.ResourceProfile{
			{
				Resource: r,
				ScopeProfiles: []pprofiletest.ScopeProfile{
					{
						SchemaURL: "test_schema_url",
						Scope:     s1,
						Profiles: []pprofiletest.Profile{
							{
								Attributes: []pprofiletest.Attribute{
									{Key: "http.method", Value: "get"},
									{Key: "http.path", Value: "/health"},
									{Key: "http.url", Value: "http://localhost/health"},
									{Key: "flags", Value: "A|B|C"},
									{Key: "total.string", Value: "123456789"},
								},
								ProfileID:              profileID,
								TimeNanos:              1111,
								DurationNanos:          222,
								DroppedAttributesCount: 1,
								OriginalPayloadFormat:  "legacy",
							},
							{
								Attributes: []pprofiletest.Attribute{
									{Key: "http.method", Value: "get"},
									{Key: "http.path", Value: "/health"},
									{Key: "http.url", Value: "http://localhost/health"},
									{Key: "flags", Value: "C|D"},
									{Key: "total.string", Value: "345678"},
								},
								ProfileID:              profileID,
								TimeNanos:              3333,
								DurationNanos:          444,
								DroppedAttributesCount: 1,
								OriginalPayloadFormat:  "non-legacy",
							},
						},
					},
					{
						SchemaURL: "test_schema_url",
						Scope:     s2,
						Profiles: []pprofiletest.Profile{
							{
								ProfileID:              profileID,
								TimeNanos:              3333,
								DurationNanos:          444,
								DroppedAttributesCount: 1,
								OriginalPayloadFormat:  "non-legacy",
							},
						},
					},
				},
			},
		},
	}
}

func constructProfiles2() pprofile.Profiles {
	td := pprofile.NewProfiles()
	rs0 := td.ResourceProfiles().AppendEmpty()
	rs0.Resource().Attributes().PutStr("host.name", "localhost")
	rs0ils0 := rs0.ScopeProfiles().AppendEmpty()
	rs0ils0.Scope().SetName("scope1")
	fillProfileOne(rs0ils0.Profiles().AppendEmpty())
	fillProfileTwo(rs0ils0.Profiles().AppendEmpty())
	rs0ils1 := rs0.ScopeProfiles().AppendEmpty()
	rs0ils1.Scope().SetName("scope2")
	fillProfileOne(rs0ils1.Profiles().AppendEmpty())
	fillProfileTwo(rs0ils1.Profiles().AppendEmpty())
	return td
}

func fillProfileOne(profile pprofile.Profile) {
	profile.SetOriginalPayloadFormat("legacy")
	profile.Samples().AppendEmpty()
}

func fillProfileTwo(profile pprofile.Profile) {
	profile.SetOriginalPayloadFormat("non-legacy")
	profile.Samples().AppendEmpty()
}
