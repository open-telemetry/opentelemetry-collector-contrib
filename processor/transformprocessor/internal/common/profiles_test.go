// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/contexts/xprofile/ottlprofile"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl/ottlfuncs"
)

func newProfileParserCollection(t *testing.T) *ProfileParserCollection {
	t.Helper()
	pc, err := NewProfileParserCollection(
		componenttest.NewNopTelemetrySettings(),
		WithProfileParser(ottlfuncs.StandardFuncs[*ottlprofile.TransformContext]()),
		WithProfileErrorMode(ottl.PropagateError),
	)
	require.NoError(t, err)
	return pc
}

func TestProfileParserCollection_ConsumeProfiles(t *testing.T) {
	tests := []struct {
		name string
		cs   ContextStatements
	}{
		{"profile context", ContextStatements{Context: Profile, Statements: []string{`set(original_payload_format, "pass")`}}},
		{"profile context with condition", ContextStatements{Context: Profile, Conditions: []string{`original_payload_format == "operationA"`}, Statements: []string{`set(original_payload_format, "pass")`}}},
		{"inferred context", ContextStatements{Statements: []string{`set(profile.original_payload_format, "pass")`}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pc := newProfileParserCollection(t)
			consumer, err := pc.ParseContextStatements(tt.cs)
			require.NoError(t, err)
			assert.Equal(t, Profile, consumer.Context())

			pd := newTestProfiles()
			require.NoError(t, consumer.ConsumeProfiles(t.Context(), pd, nil))

			profile := pd.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0)
			assert.Equal(t, "pass", profile.OriginalPayloadFormat())
		})
	}
}

func TestProfileParserCollection_ConsumeProfiles_PropagatesError(t *testing.T) {
	pc := newProfileParserCollection(t)
	consumer, err := pc.ParseContextStatements(ContextStatements{Context: Profile, Statements: []string{`set(original_payload_format, ParseJSON("1"))`}})
	require.NoError(t, err)
	require.Error(t, consumer.ConsumeProfiles(t.Context(), newTestProfiles(), nil))
}

func TestProfileParserCollection_ParseContextStatements_Error(t *testing.T) {
	pc := newProfileParserCollection(t)
	_, err := pc.ParseContextStatements(ContextStatements{Context: Profile, Statements: []string{`not a valid statement`}})
	require.Error(t, err)
}
