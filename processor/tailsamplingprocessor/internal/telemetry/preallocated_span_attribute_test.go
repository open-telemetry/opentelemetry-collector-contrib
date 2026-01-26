// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/tailsamplingprocessor/pkg/samplingpolicy"
)

func TestNewDecisionAttributes_AllDecisions(t *testing.T) {
	attrKey := attribute.Key("decision.final")
	da := NewDecisionAttributes(attrKey)

	require.NotNil(t, da)

	// Test all 8 valid decisions
	testCases := []struct {
		decision      samplingpolicy.Decision
		expectedValue string
	}{
		{samplingpolicy.Unspecified, "unspecified"},
		{samplingpolicy.Pending, "pending"},
		{samplingpolicy.Sampled, "sampled"},
		{samplingpolicy.NotSampled, "not_sampled"},
		{samplingpolicy.Dropped, "dropped"},
		{samplingpolicy.Error, "error"},
		{samplingpolicy.InvertSampled, "invert_sampled"},
		{samplingpolicy.InvertNotSampled, "invert_not_sampled"},
	}

	for _, tc := range testCases {
		t.Run(tc.expectedValue, func(t *testing.T) {
			attr := da.Get(tc.decision)
			assert.Equal(t, attrKey, attr.Key, "Key should match")
			assert.Equal(t, tc.expectedValue, attr.Value.AsString(), "Value should match decision string")
		})
	}
}

func TestDecisionAttributes_Get_UnknownDecisions(t *testing.T) {
	attrKey := attribute.Key("test.key")
	da := NewDecisionAttributes(attrKey)

	testCases := []struct {
		name     string
		decision samplingpolicy.Decision
	}{
		{"negative decision", samplingpolicy.Decision(-1)},
		{"decision 8", samplingpolicy.Decision(8)},
		{"decision 100", samplingpolicy.Decision(100)},
		{"decision 255", samplingpolicy.Decision(255)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			attr := da.Get(tc.decision)
			assert.Equal(t, attrKey, attr.Key, "Key should match")
			assert.Equal(t, "missing.preallocation", attr.Value.AsString(), "Should return unknown value")
		})
	}

	// Verify unknownAttr is consistent across multiple calls
	attr1 := da.Get(samplingpolicy.Decision(-1))
	attr2 := da.Get(samplingpolicy.Decision(100))
	assert.Equal(t, attr1.Value.AsString(), attr2.Value.AsString(), "Unknown attributes should be consistent")
}

func TestDecisionAttributes_StringConsistency(t *testing.T) {
	attrKey := attribute.Key("policy.decision")
	da := NewDecisionAttributes(attrKey)

	// Verify each decision's attribute value matches its String() method
	allDecisions := []samplingpolicy.Decision{
		samplingpolicy.Unspecified,
		samplingpolicy.Pending,
		samplingpolicy.Sampled,
		samplingpolicy.NotSampled,
		samplingpolicy.Dropped,
		samplingpolicy.Error,
		samplingpolicy.InvertSampled,
		samplingpolicy.InvertNotSampled,
	}

	for _, decision := range allDecisions {
		t.Run(decision.String(), func(t *testing.T) {
			attr := da.Get(decision)
			expectedString := decision.String()
			actualString := attr.Value.AsString()
			assert.Equal(t, expectedString, actualString,
				"Decision %d: attribute value should match String() method", decision)
		})
	}
}

func TestDecisionAttributes_DifferentKeys(t *testing.T) {
	key1 := attribute.Key("decision.final")
	key2 := attribute.Key("policy.decision")

	da1 := NewDecisionAttributes(key1)
	da2 := NewDecisionAttributes(key2)

	// Test with the same decision type but different keys
	decision := samplingpolicy.Sampled

	attr1 := da1.Get(decision)
	attr2 := da2.Get(decision)

	// Keys should be different
	assert.NotEqual(t, attr1.Key, attr2.Key, "Keys should differ")
	assert.Equal(t, key1, attr1.Key, "First attribute should use first key")
	assert.Equal(t, key2, attr2.Key, "Second attribute should use second key")

	// Values should be the same (both "sampled")
	assert.Equal(t, attr1.Value.AsString(), attr2.Value.AsString(), "Values should be the same")
	assert.Equal(t, "sampled", attr1.Value.AsString(), "Value should be 'sampled'")
}

func TestDecisionAttributes_Concurrent(t *testing.T) {
	attrKey := attribute.Key("concurrent.test")
	da := NewDecisionAttributes(attrKey)

	// Number of goroutines and iterations
	numGoroutines := 100
	iterations := 1000

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// All valid decisions
	validDecisions := []samplingpolicy.Decision{
		samplingpolicy.Unspecified,
		samplingpolicy.Pending,
		samplingpolicy.Sampled,
		samplingpolicy.NotSampled,
		samplingpolicy.Dropped,
		samplingpolicy.Error,
		samplingpolicy.InvertSampled,
		samplingpolicy.InvertNotSampled,
	}

	// Spawn multiple goroutines that concurrently call Get()
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Cycle through all valid decisions
				decision := validDecisions[j%len(validDecisions)]
				attr := da.Get(decision)

				// Verify the returned attribute is valid
				assert.Equal(t, attrKey, attr.Key)
				assert.Equal(t, decision.String(), attr.Value.AsString())
			}
		}(i)
	}

	wg.Wait()
}

func TestDecisionAttributes_ZeroValueKey(t *testing.T) {
	// Test with empty key (unusual but valid)
	emptyKey := attribute.Key("")
	da := NewDecisionAttributes(emptyKey)

	attr := da.Get(samplingpolicy.Sampled)
	assert.Equal(t, emptyKey, attr.Key, "Should work with empty key")
	assert.Equal(t, "sampled", attr.Value.AsString(), "Value should still be correct")
}

func TestDecisionAttributes_ImmutabilityAfterConstruction(t *testing.T) {
	attrKey := attribute.Key("immutable.test")
	da := NewDecisionAttributes(attrKey)

	// Get the same decision multiple times
	attr1 := da.Get(samplingpolicy.Sampled)
	attr2 := da.Get(samplingpolicy.Sampled)
	attr3 := da.Get(samplingpolicy.Sampled)

	// All should return the same value
	assert.Equal(t, attr1.Value.AsString(), attr2.Value.AsString())
	assert.Equal(t, attr2.Value.AsString(), attr3.Value.AsString())
	assert.Equal(t, "sampled", attr1.Value.AsString())
}
