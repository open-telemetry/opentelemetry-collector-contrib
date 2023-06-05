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

package sumologicexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestGetMetadata(t *testing.T) {
	attributes1 := pcommon.NewMap()
	attributes1.PutStr("key3", "to-be-overridden")
	attributes1.PutStr("key1", "value1")
	attributes1.PutStr("key2", "value2")
	attributes1.PutStr("additional_key2", "value2")
	attributes1.PutStr("additional_key3", "value3")
	attributes2 := pcommon.NewMap()
	attributes2.PutStr("additional_key1", "value1")
	attributes2.PutStr("key3", "value3")

	regexes := []string{"^key[12]", "^key3"}
	f, err := newFilter(regexes)
	require.NoError(t, err)

	metadata := f.mergeAndFilterIn(attributes1, attributes2)
	expected := fieldsFromMap(map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	})
	// Use string() because object comparison has not been reliable
	assert.Equal(t, expected.string(), metadata.string())
}

func TestFilterOutMetadata(t *testing.T) {
	attributes := pcommon.NewMap()
	attributes.PutStr("key3", "value3")
	attributes.PutStr("key1", "value1")
	attributes.PutStr("key2", "value2")
	attributes.PutStr("additional_key2", "value2")
	attributes.PutStr("additional_key3", "value3")

	regexes := []string{"^key[12]", "^key3"}
	f, err := newFilter(regexes)
	require.NoError(t, err)

	data := f.filterOut(attributes)
	expected := fieldsFromMap(map[string]string{
		"additional_key2": "value2",
		"additional_key3": "value3",
	})
	// Use string() because object comparison has not been reliable
	assert.Equal(t, expected.string(), data.string())
}
