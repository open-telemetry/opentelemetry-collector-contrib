// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package maps

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMergeStringMaps(t *testing.T) {
	m1 := map[string]string{
		"key-1": "val-1",
	}

	m2 := map[string]string{
		"key-2": "val-2",
	}

	actual := MergeStringMaps(m1, m2)
	expected := map[string]string{
		"key-1": "val-1",
		"key-2": "val-2",
	}

	require.Equal(t, expected, actual)
}

func TestCloneStringMap(t *testing.T) {
	m := map[string]string{
		"key-1": "val-1",
	}

	actual := CloneStringMap(m)
	expected := map[string]string{
		"key-1": "val-1",
	}

	require.Equal(t, expected, actual)
}
