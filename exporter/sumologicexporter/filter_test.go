// Copyright 2020 OpenTelemetry Authors
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
	"go.opentelemetry.io/collector/model/pdata"
)

func TestGetMetadata(t *testing.T) {
	attributes := pdata.NewAttributeMap()
	attributes.InsertString("key3", "value3")
	attributes.InsertString("key1", "value1")
	attributes.InsertString("key2", "value2")
	attributes.InsertString("additional_key2", "value2")
	attributes.InsertString("additional_key3", "value3")

	regexes := []string{"^key[12]", "^key3"}
	f, err := newFilter(regexes)
	require.NoError(t, err)

	metadata := f.filterIn(attributes)
	expected := fieldsFromMap(map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	})
	// Use string() because object comparison has not been reliable
	assert.Equal(t, expected.string(), metadata.string())
}

func TestFilterOutMetadata(t *testing.T) {
	attributes := pdata.NewAttributeMap()
	attributes.InsertString("key3", "value3")
	attributes.InsertString("key1", "value1")
	attributes.InsertString("key2", "value2")
	attributes.InsertString("additional_key2", "value2")
	attributes.InsertString("additional_key3", "value3")

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
