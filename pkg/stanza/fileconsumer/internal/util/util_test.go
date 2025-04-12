// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMapCopy(t *testing.T) {
	initMap := map[string]any{
		"mapVal": map[string]any{
			"nestedVal": "value1",
		},
		"intVal": 1,
		"strVal": "OrigStr",
	}

	copyMap := MapCopy(initMap)
	// Mutate values on the copied map
	copyMap["mapVal"].(map[string]any)["nestedVal"] = "overwrittenValue"
	copyMap["intVal"] = 2
	copyMap["strVal"] = "CopyString"

	// Assert that the original map should have the same values
	assert.Equal(t, "value1", initMap["mapVal"].(map[string]any)["nestedVal"])
	assert.Equal(t, 1, initMap["intVal"])
	assert.Equal(t, "OrigStr", initMap["strVal"])
}
