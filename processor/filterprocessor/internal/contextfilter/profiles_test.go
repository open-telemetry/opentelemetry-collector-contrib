// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package contextfilter_test // import "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/contextfilter_test"
import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/processor/processorhelper"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	common "github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/contextfilter"
)

func TestFilterProfileProcessorWithOTTL(t *testing.T) {
	tests := []struct {
		name             string
		conditions       []string
		filterEverything bool
		want             func(pprofile.Profiles)
		wantErr          bool
		errorMode        ottl.ErrorMode
	}{
		{
			name: "drop profiles",
			conditions: []string{
				`profile.original_payload_format == "legacy"`,
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
			conditions: []string{
				`IsMatch(profile.original_payload_format, ".*legacy")`,
			},
			filterEverything: true,
			errorMode:        ottl.IgnoreError,
		},
		{
			name: "multiple conditions",
			conditions: []string{
				`IsMatch(profile.original_payload_format, "wrong")`,
				`IsMatch(profile.original_payload_format, ".*legacy")`,
			},
			filterEverything: true,
			errorMode:        ottl.IgnoreError,
		},
		{
			name: "with error conditions",
			conditions: []string{
				`Substring("", 0, 100) == "test"`,
			},
			want:      func(_ pprofile.Profiles) {},
			wantErr:   true,
			errorMode: ottl.IgnoreError,
		},
		{
			name: "filters resource",
			conditions: []string{
				`resource.attributes["host.name"] == "localhost"`,
			},
			filterEverything: true,
			errorMode:        ottl.IgnoreError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collection, err := common.NewProfileParserCollection(componenttest.NewNopTelemetrySettings(), common.WithProfileParser(filterottl.StandardProfileFuncs()))
			assert.NoError(t, err)
			got, err := collection.ParseContextConditions(common.ContextConditions{Conditions: tt.conditions, ErrorMode: tt.errorMode})
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err, "error parsing conditions")
			finalProfiles := constructProfiles()
			consumeErr := got.ConsumeProfiles(context.Background(), finalProfiles)
			switch {
			case tt.filterEverything && !tt.wantErr:
				assert.Equal(t, processorhelper.ErrSkipProcessingData, consumeErr)
			case tt.wantErr:
				assert.Error(t, consumeErr)
			default:
				assert.NoError(t, consumeErr)
				exTd := constructProfiles()
				tt.want(exTd)
				assert.Equal(t, exTd, finalProfiles)
			}
		})
	}
}

func Test_ProcessProfiles_ConditionsErrorMode(t *testing.T) {
	tests := []struct {
		name          string
		errorMode     ottl.ErrorMode
		conditions    []common.ContextConditions
		want          func(pprofile.Profiles)
		wantErr       bool
		wantErrorWith string
	}{
		{
			name:      "profile: conditions group with error mode",
			errorMode: ottl.PropagateError,
			conditions: []common.ContextConditions{
				{Conditions: []string{`profile.original_payload_format == ParseJSON("1")`}, ErrorMode: ottl.IgnoreError},
				{Conditions: []string{`IsMatch(profile.original_payload_format, "non-legacy")`}},
			},
			want: func(pd pprofile.Profiles) {
				pd.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().RemoveIf(func(profile pprofile.Profile) bool {
					return profile.OriginalPayloadFormat() == "non-legacy"
				})
				pd.ResourceProfiles().At(0).ScopeProfiles().At(1).Profiles().RemoveIf(func(profile pprofile.Profile) bool {
					return profile.OriginalPayloadFormat() == "non-legacy"
				})
			},
		},
		{
			name:      "profile: conditions group error mode does not affect default",
			errorMode: ottl.PropagateError,
			conditions: []common.ContextConditions{
				{Conditions: []string{`profile.original_payload_format == ParseJSON("1")`}, ErrorMode: ottl.IgnoreError},
				{Conditions: []string{`profile.original_payload_format == ParseJSON(true)`}},
			},
			wantErr:       true,
			wantErrorWith: "expected string but got bool",
		},
		{
			name:      "resource: conditions group with error mode",
			errorMode: ottl.PropagateError,
			conditions: []common.ContextConditions{
				{Conditions: []string{`resource.attributes["pass"] == ParseJSON("1")`}, ErrorMode: ottl.IgnoreError},
				{Conditions: []string{`not IsMatch(resource.attributes["host.name"], ".*")`}},
			},
			want: func(pd pprofile.Profiles) {
				pd.ResourceProfiles().RemoveIf(func(rl pprofile.ResourceProfiles) bool {
					v, _ := rl.Resource().Attributes().Get("host.name")
					return len(v.AsString()) == 0
				})
			},
		},
		{
			name:      "resource: conditions group error mode does not affect default",
			errorMode: ottl.PropagateError,
			conditions: []common.ContextConditions{
				{Conditions: []string{`resource.attributes["pass"] == ParseJSON("1")`}, ErrorMode: ottl.IgnoreError},
				{Conditions: []string{`resource.attributes["pass"] == ParseJSON(true)`}},
			},
			wantErr:       true,
			wantErrorWith: "expected string but got bool",
		},
		{
			name:      "scope: conditions group with error mode",
			errorMode: ottl.PropagateError,
			conditions: []common.ContextConditions{
				{Conditions: []string{`scope.name == ParseJSON("1")`}, ErrorMode: ottl.IgnoreError},
				{Conditions: []string{`scope.name == "scope1"`}},
			},
			want: func(pd pprofile.Profiles) {
				pd.ResourceProfiles().RemoveIf(func(rl pprofile.ResourceProfiles) bool {
					rl.ScopeProfiles().RemoveIf(func(sl pprofile.ScopeProfiles) bool {
						return sl.Scope().Name() == "scope1"
					})
					return rl.ScopeProfiles().Len() == 0
				})
			},
		},
		{
			name:      "scope: conditions group error mode does not affect default",
			errorMode: ottl.PropagateError,
			conditions: []common.ContextConditions{
				{Conditions: []string{`scope.name == ParseJSON("1")`}, ErrorMode: ottl.IgnoreError},
				{Conditions: []string{`scope.name == ParseJSON(true)`}},
			},
			wantErr:       true,
			wantErrorWith: "expected string but got bool",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collection, err := common.NewProfileParserCollection(componenttest.NewNopTelemetrySettings(), common.WithProfileParser(filterottl.StandardProfileFuncs()), common.WithProfileErrorMode(tt.errorMode))
			assert.NoError(t, err)

			var consumers []common.ProfilesConsumer
			var parseErrs []error
			for _, condition := range tt.conditions {
				consumer, err := collection.ParseContextConditions(condition)
				parseErrs = append(parseErrs, err)
				consumers = append(consumers, consumer)
			}

			if tt.wantErr {
				found := false
				for _, e := range parseErrs {
					if e != nil && strings.Contains(e.Error(), tt.wantErrorWith) {
						found = true
						assert.True(t, found)
						return
					}
				}
			}

			for _, e := range parseErrs {
				assert.NoError(t, e, "error parsing conditions")
			}

			finalProfiles := constructProfiles()
			var consumeErr error

			// Apply each consumer sequentially
			for _, consumer := range consumers {
				if err := consumer.ConsumeProfiles(context.Background(), finalProfiles); err != nil {
					if errors.Is(err, processorhelper.ErrSkipProcessingData) {
						consumeErr = err
						break
					}
					consumeErr = err
					break
				}
			}

			if consumeErr != nil && errors.Is(consumeErr, processorhelper.ErrSkipProcessingData) {
				assert.NoError(t, consumeErr)
				return
			}

			exTd := constructProfiles()
			tt.want(exTd)
			assert.Equal(t, exTd, finalProfiles)
		})
	}
}

func Test_ProcessProfiles_InferredResourceContext(t *testing.T) {
	tests := []struct {
		condition          string
		filteredEverything bool
		want               func(pprofile.Profiles)
	}{
		{
			condition:          `resource.attributes["host.name"] == "localhost"`,
			filteredEverything: true,
			want: func(_ pprofile.Profiles) {
				// Everything should be filtered out
			},
		},
		{
			condition:          `resource.attributes["host.name"] == "wrong"`,
			filteredEverything: false,
			want: func(_ pprofile.Profiles) {
				// Nothing should be filtered, original data remains
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.condition, func(t *testing.T) {
			td := constructProfiles()

			collection, err := common.NewProfileParserCollection(componenttest.NewNopTelemetrySettings(), common.WithProfileParser(filterottl.StandardProfileFuncs()))
			assert.NoError(t, err)

			consumer, err := collection.ParseContextConditions(common.ContextConditions{Conditions: []string{tt.condition}})
			assert.NoError(t, err)

			err = consumer.ConsumeProfiles(context.Background(), td)

			if tt.filteredEverything {
				assert.Equal(t, processorhelper.ErrSkipProcessingData, err)
			} else {
				assert.NoError(t, err)
				exTd := constructProfiles()
				tt.want(exTd)
				assert.Equal(t, exTd, td)
			}
		})
	}
}

func Test_ProcessProfiles_InferredScopeContext(t *testing.T) {
	tests := []struct {
		condition          string
		filteredEverything bool
		want               func(pprofile.Profiles)
	}{
		{
			//condition:          `scope.name == "scope"`,
			condition:          `IsMatch(scope.name, "scope*")`,
			filteredEverything: true,
			want: func(_ pprofile.Profiles) {
				// Everything should be filtered out since scope name matches
			},
		},
		{
			condition:          `scope.version == "2"`,
			filteredEverything: false,
			want: func(_ pprofile.Profiles) {
				// Nothing should be filtered, original data remains
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.condition, func(t *testing.T) {
			td := constructProfiles()

			collection, err := common.NewProfileParserCollection(componenttest.NewNopTelemetrySettings(), common.WithProfileParser(filterottl.StandardProfileFuncs()))
			assert.NoError(t, err)

			consumer, err := collection.ParseContextConditions(common.ContextConditions{Conditions: []string{tt.condition}})
			assert.NoError(t, err)

			err = consumer.ConsumeProfiles(context.Background(), td)

			if tt.filteredEverything {
				assert.Equal(t, processorhelper.ErrSkipProcessingData, err)
			} else {
				assert.NoError(t, err)
				exTd := constructProfiles()
				tt.want(exTd)
				assert.Equal(t, exTd, td)
			}
		})
	}
}

func constructProfiles() pprofile.Profiles {
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
