// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package sumologicexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFieldsAsString(t *testing.T) {
	expected := "key1=value1, key2=value2, key3=value3"
	flds := fieldsFromMap(map[string]string{
		"key1": "value1",
		"key3": "value3",
		"key2": "value2",
	})

	assert.Equal(t, expected, flds.string())
}

func TestFieldsSanitization(t *testing.T) {
	expected := "key1=value_1, key3=value_3, key:_2=valu_e:2"
	flds := fieldsFromMap(map[string]string{
		"key1":   "value,1",
		"key3":   "value\n3",
		"key=,2": "valu,e=2",
	})

	assert.Equal(t, expected, flds.string())
}
