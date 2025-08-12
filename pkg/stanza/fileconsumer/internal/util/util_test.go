// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestIsGzipFile(t *testing.T) {

	temp, err := os.Create(filepath.Join(t.TempDir(), "test.log"))
	require.NoError(t, err)

	tempWrite := gzip.NewWriter(temp)
	_, err = tempWrite.Write([]byte("this is test data and the header should prove this is gzip"))
	require.NoError(t, err)
	tempWrite.Close()

	// set offset to start
	temp.Seek(0, io.SeekStart)

	require.True(t, IsGzipFile(temp))
}
