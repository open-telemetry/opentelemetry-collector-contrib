// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package awsecsattributesprocessor

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMetadataFlatten(t *testing.T) {
	var md containerMetadata
	require.NoError(t, json.Unmarshal([]byte(payload), &md))
	require.Equal(t, expectedFlattenedMetadata, md.flat())
}

func TestMetadataFlattenEmptyLabels(t *testing.T) {
	// A document with nil Labels must not panic and must still emit the base keys.
	md := containerMetadata{DockerID: "abc", Image: "img:latest"}
	flat := md.flat()
	require.Equal(t, "abc", flat["docker.id"])
	require.Equal(t, "img:latest", flat["image"])
	// No labels means no labels.* keys.
	for k := range flat {
		require.NotContains(t, k, "labels.")
	}
}
