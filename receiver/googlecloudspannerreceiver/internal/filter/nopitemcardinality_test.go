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

package filter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNopItemCardinalityFilter_Filter(t *testing.T) {
	filter := NewNopItemCardinalityFilter()
	sourceItems := []*Item{{}}

	filteredItems, err := filter.Filter(sourceItems)

	require.NoError(t, err)
	assert.Equal(t, sourceItems, filteredItems)
}

func TestNopItemCardinalityFilter_Shutdown(t *testing.T) {
	filter := &nopItemCardinalityFilter{}

	err := filter.Shutdown()

	require.NoError(t, err)
}

func TestNopItemCardinalityFilter_TotalLimit(t *testing.T) {
	filter := &nopItemCardinalityFilter{}

	assert.Equal(t, 0, filter.TotalLimit())
}

func TestNopItemCardinalityFilter_LimitByTimestamp(t *testing.T) {
	filter := &nopItemCardinalityFilter{}

	assert.Equal(t, 0, filter.LimitByTimestamp())
}

func TestNopItemFilterResolver_Resolve(t *testing.T) {
	itemFilterResolver := NewNopItemFilterResolver()

	itemFilter, err := itemFilterResolver.Resolve("test")

	require.NoError(t, err)
	assert.Equal(t, &nopItemCardinalityFilter{}, itemFilter)
}

func TestNopItemFilterResolver_Shutdown(t *testing.T) {
	itemFilterResolver := NewNopItemFilterResolver()

	err := itemFilterResolver.Shutdown()

	require.NoError(t, err)
}
