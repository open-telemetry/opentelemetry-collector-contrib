// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

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
