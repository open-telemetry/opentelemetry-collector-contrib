// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package partitioningprocessor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/pdata/pprofile"
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
	pd := pprofile.NewProfiles()
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
		counts[pp.values[0].value] = pp.data.ResourceProfiles().Len()
	}
	assert.Equal(t, 2, counts["t1"])
	assert.Equal(t, 1, counts["t2"])
}

func TestPartitionProfiles_ScopePartitioning(t *testing.T) {
	pd := pprofile.NewProfiles()
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
		scopes[pp.values[0].value] = true
	}
	assert.True(t, scopes["scope-a"])
	assert.True(t, scopes["scope-b"])
}

func TestPartitionProfiles_ProfilePartitioning(t *testing.T) {
	pd := pprofile.NewProfiles()
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
		require.Equal(t, 1, pp.data.ResourceProfiles().Len())
		require.Equal(t, 1, pp.data.ResourceProfiles().At(0).ScopeProfiles().Len())
		counts[pp.values[0].value] = pp.data.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().Len()
	}
	assert.Equal(t, 2, counts["pprof-a"])
	assert.Equal(t, 1, counts["pprof-b"])
}

func TestPartitionProfiles_EmptyInput(t *testing.T) {
	result := partitionProfiles(t, pprofile.NewProfiles(),
		`resource.attributes["tenant.id"]`,
	)
	assert.Empty(t, result)
}

func TestPartitionProfiles_NilAttributeValue(t *testing.T) {
	pd := pprofile.NewProfiles()
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
	pd := pprofile.NewProfiles()
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
	pd := pprofile.NewProfiles()
	dict := pd.Dictionary()
	dict.StringTable().Append("attr.key")
	dict.StringTable().Append("unit")
	attr := dict.AttributeTable().AppendEmpty()
	attr.SetKeyStrindex(0)
	attr.SetUnitStrindex(1)

	rp := pd.ResourceProfiles().AppendEmpty()
	rp.Resource().Attributes().PutStr("tenant.id", "t1")
	sp := rp.ScopeProfiles().AppendEmpty()
	profile := sp.Profiles().AppendEmpty()
	profile.AttributeIndices().Append(0)

	result := partitionProfiles(t, pd,
		`resource.attributes["tenant.id"]`,
	)
	require.Len(t, result, 1)

	got := result[0].data
	assert.Equal(t, 2, got.Dictionary().StringTable().Len(), "partitioned profiles should retain referenced dictionary strings")
	assert.Equal(t, 1, got.Dictionary().AttributeTable().Len(), "partitioned profiles should retain referenced dictionary attributes")
	assert.Equal(t, int32(0), got.ResourceProfiles().At(0).ScopeProfiles().At(0).Profiles().At(0).AttributeIndices().At(0))
}

func partitionProfiles(t *testing.T, pd pprofile.Profiles, expressions ...string) []partitionedProfiles {
	t.Helper()
	p, err := newProfilesPartitioner(expressions, componenttest.NewNopTelemetrySettings())
	require.NoError(t, err)
	result, err := p.partitionProfiles(t.Context(), pd)
	require.NoError(t, err)
	return result
}
