// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
