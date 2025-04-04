// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package golden

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestSortAttributes(t *testing.T) {
	// Create a sample input map
	mp := pcommon.NewMap()
	mp.PutStr("name", "John")
	mp.PutInt("age", 30)
	mp.PutBool("isStudent", false)
	subMap := pcommon.NewMap()
	subMap.PutStr("city", "New York")
	subMap.CopyTo(mp.PutEmptyMap("location"))

	// Call the sortAttributeMap method
	sortAttributeMap(mp)

	// Verify the sorted keys
	expectedKeys := []string{"age", "isStudent", "location", "name"}
	actualKeys := []string{}
	for key := range mp.All() {
		actualKeys = append(actualKeys, key)
	}

	// Check if the keys are sorted correctly
	if len(expectedKeys) != len(actualKeys) {
		t.Errorf("Incorrect number of keys. Expected: %d, Actual: %d", len(expectedKeys), len(actualKeys))
	}

	for i, key := range expectedKeys {
		if key != actualKeys[i] {
			t.Errorf("Incorrect key at index %d. Expected: %s, Actual: %s", i, key, actualKeys[i])
		}
	}
}

func TestSortMetricsResourceAndScope(t *testing.T) {
	beforePath := filepath.Join("testdata", "sort-metrics", "before.yaml")
	afterPath := filepath.Join("testdata", "sort-metrics", "after.yaml")
	before, err := ReadMetrics(beforePath)
	require.NoError(t, err)
	sortResources(before)
	sortScopes(before)
	sortMetricDataPointSlices(before)
	after, err := ReadMetrics(afterPath)
	require.NoError(t, err)
	require.Equal(t, before, after)
}
