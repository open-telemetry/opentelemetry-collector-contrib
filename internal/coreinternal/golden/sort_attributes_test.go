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
	mp.Range(func(key string, _ pcommon.Value) bool {
		actualKeys = append(actualKeys, key)
		return true
	})

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

func TestSortMetricDatapointSlice(t *testing.T) {
	dir := filepath.Join("testdata", "sort-datapoint-slice")
	before, err := ReadMetrics(filepath.Join(dir, "before.yaml"))
	require.NoError(t, err)
	after, err := ReadMetrics(filepath.Join(dir, "after.yaml"))
	require.NoError(t, err)

	require.Equal(t, before, after)
}
