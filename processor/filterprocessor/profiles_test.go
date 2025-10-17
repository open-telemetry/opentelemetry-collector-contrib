// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package filterprocessor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/processor/processorhelper"
	"go.opentelemetry.io/collector/processor/processortest"
	"go.opentelemetry.io/collector/processor/xprocessor"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/sdk/metric/metricdata/metricdatatest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/metadata"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor/internal/metadatatest"
)

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
		name             string
		conditions       []string
		filterEverything bool
		want             func(pprofile.Profiles)
		errorMode        ottl.ErrorMode
	}{
		{
			name: "drop profiles",
			conditions: []string{
				`original_payload_format == "legacy"`,
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
				`IsMatch(original_payload_format, ".*legacy")`,
			},
			filterEverything: true,
			errorMode:        ottl.IgnoreError,
		},
		{
			name: "multiple conditions",
			conditions: []string{
				`IsMatch(original_payload_format, "wrong name")`,
				`IsMatch(original_payload_format, ".*legacy")`,
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
			errorMode: ottl.IgnoreError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Profiles: ProfileFilters{ProfileConditions: tt.conditions}, profileFunctions: defaultProfileFunctionsMap()}
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

	_, err = processor.processProfiles(t.Context(), constructProfiles())
	assert.NoError(t, err)

	metadatatest.AssertEqualProcessorFilterProfilesFiltered(t, tel, []metricdata.DataPoint[int64]{
		{
			Value:      2,
			Attributes: attribute.NewSet(attribute.String("filter", "filter")),
		},
	}, metricdatatest.IgnoreTimestamp())
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
	profile.Sample().AppendEmpty()
}

func fillProfileTwo(profile pprofile.Profile) {
	profile.SetOriginalPayloadFormat("non-legacy")
	profile.Sample().AppendEmpty()
}
