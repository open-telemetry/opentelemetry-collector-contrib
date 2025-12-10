package awsecsattributesprocessor

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"gotest.tools/v3/assert"
)

func TestMetadataFlatten(t *testing.T) {
	var metadata Metadata
	require.NoError(t, json.Unmarshal([]byte(payload), &metadata))
	assert.DeepEqual(t, expectedFlattenedMetadata, metadata.Flat())
}
