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
	require.Equal(t, "abc", flat["container.id"])
	require.Equal(t, "img:latest", flat["container.image.name"])
	// No labels means no container.label.* keys.
	for k := range flat {
		require.NotContains(t, k, "container.label.")
	}
}
