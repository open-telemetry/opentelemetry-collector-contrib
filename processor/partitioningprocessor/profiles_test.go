// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package partitioningprocessor

import (
	"testing"

	"github.com/open-telemetry/sig-profiling/profcheck"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.opentelemetry.io/collector/pdata/testdata"
	otlpprofiles "go.opentelemetry.io/proto/otlp/profiles/v1development"
	"google.golang.org/protobuf/proto"
)

// --- parser tests ---

func TestNewProfilesPartitioner_ResourceContext(t *testing.T) {
	expressions := []string{`resource.attributes["tenant.id"]`}
	p, err := newProfilesPartitioner(expressions, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	_, ok := p.(*resourceProfilesPartitioner)
	assert.True(t, ok)
}

func TestNewProfilesPartitioner_ScopeContext(t *testing.T) {
	expressions := []string{`scope.name`}
	p, err := newProfilesPartitioner(expressions, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	_, ok := p.(*scopeProfilesPartitioner)
	assert.True(t, ok)
}

func TestNewProfilesPartitioner_ProfileContext(t *testing.T) {
	expressions := []string{`profile.attributes["x"]`}
	p, err := newProfilesPartitioner(expressions, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	_, ok := p.(*profileProfilesPartitioner)
	assert.True(t, ok)
}

func TestNewProfilesPartitioner_OTelColContext(t *testing.T) {
	expressions := []string{`otelcol.client.metadata["x-tenant-id"][0]`}
	p, err := newProfilesPartitioner(expressions, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	_, ok := p.(*otelcolProfilesPartitioner)
	assert.True(t, ok)
}

func TestNewProfilesPartitioner_RejectsProfileSample(t *testing.T) {
	expressions := []string{`profilesample.value`}
	_, err := newProfilesPartitioner(expressions, componenttest.NewNopTelemetrySettings())
	assert.Error(t, err, "profilesample context should be rejected")
}

func TestNewProfilesPartitioner_InvalidExpression(t *testing.T) {
	expressions := []string{`not_a_valid_expression(`}
	_, err := newProfilesPartitioner(expressions, componenttest.NewNopTelemetrySettings())
	assert.Error(t, err)
}

// --- partitioner tests ---

func TestPartitionProfiles_ResourcePartitioning(t *testing.T) {
	pd := newTestProfiles()
	for _, tenant := range []string{"t1", "t2", "t1"} {
		rp := pd.ResourceProfiles().AppendEmpty()
		rp.Resource().Attributes().PutStr("tenant.id", tenant)
		rp.ScopeProfiles().AppendEmpty().Profiles().AppendEmpty()
	}

	result := partitionProfiles(t, pd,
		`resource.attributes["tenant.id"]`,
	)
	require.Len(t, result, 2)

	counts := make(map[string]int)
	for _, pp := range result {
		require.Len(t, pp.values, 1)
		requireConformant(t, pp.data)
		counts[pp.values[0].value] = pp.data.ResourceProfiles().Len()
	}
	assert.Equal(t, 2, counts["t1"])
	assert.Equal(t, 1, counts["t2"])
}

func TestPartitionProfiles_ScopePartitioning(t *testing.T) {
	pd := newTestProfiles()
	rp := pd.ResourceProfiles().AppendEmpty()

	sp1 := rp.ScopeProfiles().AppendEmpty()
	sp1.Scope().SetName("scope-a")
	sp1.Profiles().AppendEmpty()

	sp2 := rp.ScopeProfiles().AppendEmpty()
	sp2.Scope().SetName("scope-b")
	sp2.Profiles().AppendEmpty()

	result := partitionProfiles(t, pd,
		`scope.name`,
	)
	require.Len(t, result, 2)

	scopes := make(map[string]bool)
	for _, pp := range result {
		require.Len(t, pp.values, 1)
		requireConformant(t, pp.data)
		scopes[pp.values[0].value] = true
	}
	assert.True(t, scopes["scope-a"])
	assert.True(t, scopes["scope-b"])
}

func TestPartitionProfiles_ProfilePartitioning(t *testing.T) {
	pd := newTestProfiles()
	rp := pd.ResourceProfiles().AppendEmpty()
	sp := rp.ScopeProfiles().AppendEmpty()

	for _, format := range []string{"pprof-a", "pprof-b", "pprof-a"} {
		sp.Profiles().AppendEmpty().SetOriginalPayloadFormat(format)
	}

	result := partitionProfiles(t, pd,
		`profile.original_payload_format`,
	)
	require.Len(t, result, 2)

	counts := make(map[string]int)
	for _, pp := range result {
		require.Len(t, pp.values, 1)
		requireConformant(t, pp.data)
		require.Equal(t, 1, pp.data.ResourceProfiles().Len())
		require.Equal(t, 1, pp.data.ResourceProfiles().At(0).ScopeProfiles().Len())
		counts[pp.values[0].value] = pp.data.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().Len()
	}
	assert.Equal(t, 2, counts["pprof-a"])
	assert.Equal(t, 1, counts["pprof-b"])
}

func TestPartitionProfiles_EmptyInput(t *testing.T) {
	result := partitionProfiles(t, newTestProfiles(),
		`resource.attributes["tenant.id"]`,
	)
	assert.Empty(t, result)
}

func TestPartitionProfiles_NilAttributeValue(t *testing.T) {
	pd := newTestProfiles()
	rp := pd.ResourceProfiles().AppendEmpty()
	rp.ScopeProfiles().AppendEmpty().Profiles().AppendEmpty()

	result := partitionProfiles(t, pd,
		`resource.attributes["missing"]`,
	)
	require.Len(t, result, 1)
	require.Len(t, result[0].values, 1)
	assert.True(t, result[0].values[0].isNil)
}

func TestPartitionProfiles_PreservesSchemaURL(t *testing.T) {
	pd := newTestProfiles()
	rp := pd.ResourceProfiles().AppendEmpty()
	rp.SetSchemaUrl("https://example.com/resource-schema")
	sp := rp.ScopeProfiles().AppendEmpty()
	sp.SetSchemaUrl("https://example.com/scope-schema")
	sp.Scope().SetName("s")
	sp.Profiles().AppendEmpty()

	result := partitionProfiles(t, pd,
		`scope.name`,
	)
	require.Len(t, result, 1)
	destRP := result[0].data.ResourceProfiles().At(0)
	assert.Equal(t, "https://example.com/resource-schema", destRP.SchemaUrl())
	assert.Equal(t, "https://example.com/scope-schema", destRP.ScopeProfiles().At(0).SchemaUrl())
}

func TestPartitionProfiles_PreservesDictionary(t *testing.T) {
	pd := newTestProfiles()
	dict := pd.Dictionary()
	keyIdx := int32(dict.StringTable().Len())
	dict.StringTable().Append("attr.key", "unit")
	attrIdx := int32(dict.AttributeTable().Len())
	attr := dict.AttributeTable().AppendEmpty()
	attr.SetKeyStrindex(keyIdx)
	attr.SetUnitStrindex(keyIdx + 1)
	attr.Value().SetStr("v")

	rp := pd.ResourceProfiles().AppendEmpty()
	rp.Resource().Attributes().PutStr("tenant.id", "t1")
	sp := rp.ScopeProfiles().AppendEmpty()
	profile := sp.Profiles().AppendEmpty()
	profile.AttributeIndices().Append(attrIdx)

	result := partitionProfiles(t, pd,
		`resource.attributes["tenant.id"]`,
	)
	require.Len(t, result, 1)

	got := result[0].data
	assert.Equal(t, dict.StringTable().Len(), got.Dictionary().StringTable().Len(), "partitioned profiles should retain referenced dictionary strings")
	assert.Equal(t, dict.AttributeTable().Len(), got.Dictionary().AttributeTable().Len(), "partitioned profiles should retain referenced dictionary attributes")
	assert.Equal(t, attrIdx, got.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).AttributeIndices().At(0))
	requireConformant(t, got)
}

func partitionProfiles(t *testing.T, pd pprofile.Profiles, expressions ...string) []partitionedProfiles {
	t.Helper()
	p, err := newProfilesPartitioner(expressions, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	result, err := p.partitionProfiles(t.Context(), pd)
	require.NoError(t, err)
	return result
}

// newTestProfiles returns Profiles with a conventional dictionary (an empty
// entry at index 0 of every table) and no resource profiles.
func newTestProfiles() pprofile.Profiles {
	pd := testdata.GenerateProfiles(0)
	pd.ResourceProfiles().RemoveIf(func(pprofile.ResourceProfiles) bool { return true })
	return pd
}

// requireConformant checks pd against the profiling SIG conformance checker.
// The pinned profcheck does not yet check for unreferenced dictionary entries,
// so it cannot detect the entries each partition inherits from the full input
// dictionary (see the README's known limitations).
func requireConformant(t *testing.T, pd pprofile.Profiles) {
	t.Helper()
	b, err := (&pprofile.ProtoMarshaler{}).MarshalProfiles(pd)
	require.NoError(t, err)
	var data otlpprofiles.ProfilesData
	require.NoError(t, proto.Unmarshal(b, &data))
	require.NoError(t, profcheck.ConformanceChecker{CheckDictionaryDuplicates: true}.Check(&data))
}
