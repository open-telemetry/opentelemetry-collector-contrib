// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package experimentalmetricmetadata

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockMetadataExporter struct{}

func (m *mockMetadataExporter) ConsumeMetadata(metadata []*MetadataUpdate) error {
	return nil
}

var _ MetadataExporter = (*mockMetadataExporter)(nil)

func TestResourceID(t *testing.T) {
	rid := ResourceID("someid")
	assert.EqualValues(t, "someid", rid)
}

func TestMetadataDelta(t *testing.T) {
	md := MetadataDelta{}
	assert.Empty(t, md.MetadataToAdd)
	assert.Empty(t, md.MetadataToRemove)
	assert.Empty(t, md.MetadataToUpdate)
}

func TestMetadataUpdate(t *testing.T) {
	mu := MetadataUpdate{}
	assert.Empty(t, mu.ResourceIDKey)
	assert.Empty(t, mu.ResourceID)
	assert.Empty(t, mu.MetadataToAdd)
	assert.Empty(t, mu.MetadataToRemove)
	assert.Empty(t, mu.MetadataToUpdate)
}
