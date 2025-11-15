// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package hydrolixexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestConvertAttributes(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("string_key", "string_value")
	attrs.PutInt("int_key", 42)
	attrs.PutDouble("double_key", 3.14)
	attrs.PutBool("bool_key", true)

	result := convertAttributes(attrs)

	assert.Len(t, result, 4)

	// Create a map for easy lookup
	attrMap := make(map[string]interface{})
	for _, tag := range result {
		for k, v := range tag {
			attrMap[k] = v
		}
	}

	assert.Equal(t, "string_value", attrMap["string_key"])
	assert.Equal(t, "42", attrMap["int_key"])
	assert.Equal(t, "3.140000", attrMap["double_key"])
	assert.Equal(t, "true", attrMap["bool_key"])
}

func TestConvertAttributesEmpty(t *testing.T) {
	attrs := pcommon.NewMap()
	result := convertAttributes(attrs)
	assert.Empty(t, result)
}

func TestAttributeValueToInterface(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() pcommon.Value
		expected string
	}{
		{
			name: "string value",
			setup: func() pcommon.Value {
				v := pcommon.NewValueStr("test")
				return v
			},
			expected: "test",
		},
		{
			name: "int value",
			setup: func() pcommon.Value {
				v := pcommon.NewValueInt(123)
				return v
			},
			expected: "123",
		},
		{
			name: "double value",
			setup: func() pcommon.Value {
				v := pcommon.NewValueDouble(123.45)
				return v
			},
			expected: "123.450000",
		},
		{
			name: "bool value true",
			setup: func() pcommon.Value {
				v := pcommon.NewValueBool(true)
				return v
			},
			expected: "true",
		},
		{
			name: "bool value false",
			setup: func() pcommon.Value {
				v := pcommon.NewValueBool(false)
				return v
			},
			expected: "false",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := tt.setup()
			result := attributeValueToInterface(value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAttributeValueToInterface_Bytes(t *testing.T) {
	value := pcommon.NewValueBytes()
	value.Bytes().FromRaw([]byte{1, 2, 3, 4})

	result := attributeValueToInterface(value)
	assert.Equal(t, []byte{1, 2, 3, 4}, result)
}

func TestExtractStringAttr(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("existing_key", "value")
	attrs.PutInt("int_key", 42)

	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{
			name:     "existing string attribute",
			key:      "existing_key",
			expected: "value",
		},
		{
			name:     "non-existing attribute",
			key:      "non_existing_key",
			expected: "",
		},
		{
			name:     "int attribute converted to string",
			key:      "int_key",
			expected: "42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractStringAttr(attrs, tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractStringAttrEmpty(t *testing.T) {
	attrs := pcommon.NewMap()
	result := extractStringAttr(attrs, "any_key")
	assert.Equal(t, "", result)
}

func TestConvertAttributesWithSpecialCharacters(t *testing.T) {
	attrs := pcommon.NewMap()
	attrs.PutStr("special.key", "special/value")
	attrs.PutStr("unicode_key", "测试")

	result := convertAttributes(attrs)

	assert.Len(t, result, 2)

	attrMap := make(map[string]interface{})
	for _, tag := range result {
		for k, v := range tag {
			attrMap[k] = v
		}
	}

	assert.Equal(t, "special/value", attrMap["special.key"])
	assert.Equal(t, "测试", attrMap["unicode_key"])
}

func TestConvertAttributesWithLargeValues(t *testing.T) {
	attrs := pcommon.NewMap()
	largeString := string(make([]byte, 10000))
	attrs.PutStr("large_key", largeString)

	result := convertAttributes(attrs)

	assert.Len(t, result, 1)
	assert.Contains(t, result[0], "large_key")
	assert.Equal(t, largeString, result[0]["large_key"])
}
