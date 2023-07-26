// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package entry

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCopyValueString(t *testing.T) {
	value := "test"
	require.Equal(t, "test", copyValue(value))
}

func TestCopyValueBool(t *testing.T) {
	value := true
	require.Equal(t, true, copyValue(value))
}

func TestCopyValueInt(t *testing.T) {
	value := 5
	require.Equal(t, 5, copyValue(value))
}

func TestCopyValueByte(t *testing.T) {
	value := []byte("test")[0]
	require.Equal(t, []byte("test")[0], copyValue(value))
}

func TestCopyValueNil(t *testing.T) {
	var value interface{}
	require.Equal(t, nil, copyValue(value))
}

func TestCopyValueStringArray(t *testing.T) {
	value := []string{"test"}
	require.Equal(t, value, copyValue(value))
}

func TestCopyValueIntArray(t *testing.T) {
	value := []int{5}
	require.Equal(t, value, copyValue(value))
}

func TestCopyValueByteArray(t *testing.T) {
	value := []byte("x")
	require.Equal(t, value, copyValue(value))
}

func TestCopyValueInterfaceArray(t *testing.T) {
	value := []interface{}{"test", true, 5}
	require.Equal(t, value, copyValue(value))
}

func TestCopyValueStringMap(t *testing.T) {
	value := map[string]string{"test": "value"}
	require.Equal(t, value, copyValue(value))
}

func TestCopyValueInterfaceMap(t *testing.T) {
	value := map[string]interface{}{"test": 5}
	require.Equal(t, value, copyValue(value))
}

func TestCopyValueUnknown(t *testing.T) {
	type testStruct struct {
		Test string
	}
	unknownValue := testStruct{
		Test: "value",
	}
	copiedValue := copyValue(unknownValue)
	expectedValue := map[string]interface{}{
		"Test": "value",
	}
	require.Equal(t, expectedValue, copiedValue)
}

func TestCopyStringMap(t *testing.T) {
	stringMap := map[string]string{
		"message": "test",
	}
	copiedMap := copyStringMap(stringMap)
	delete(stringMap, "message")
	require.Equal(t, "test", copiedMap["message"])
}

func TestCopyInterfaceMap(t *testing.T) {
	stringMap := map[string]interface{}{
		"message": "test",
	}
	copiedMap := copyInterfaceMap(stringMap)
	delete(stringMap, "message")
	require.Equal(t, "test", copiedMap["message"])
}

func TestCopyStringArray(t *testing.T) {
	stringArray := []string{"test"}
	copiedArray := copyStringArray(stringArray)
	stringArray[0] = "new"
	require.Equal(t, []string{"test"}, copiedArray)
}

func TestCopyByteArray(t *testing.T) {
	byteArray := []byte("test")
	copiedArray := copyByteArray(byteArray)
	byteArray[0] = 'x'
	require.Equal(t, []byte("test"), copiedArray)
}

func TestCopyIntArray(t *testing.T) {
	intArray := []int{1}
	copiedArray := copyIntArray(intArray)
	intArray[0] = 0
	require.Equal(t, []int{1}, copiedArray)
}

func TestCopyInterfaceArray(t *testing.T) {
	interfaceArray := []interface{}{"test", 0, true}
	copiedArray := copyInterfaceArray(interfaceArray)
	interfaceArray[0] = "new"
	require.Equal(t, []interface{}{"test", 0, true}, copiedArray)
}

func TestCopyUnknownValueValid(t *testing.T) {
	type testStruct struct {
		Test string
	}
	unknownValue := testStruct{
		Test: "value",
	}
	copiedValue := copyUnknown(unknownValue)
	expectedValue := map[string]interface{}{
		"Test": "value",
	}
	require.Equal(t, expectedValue, copiedValue)
}

func TestCopyUnknownValueInalid(t *testing.T) {
	unknownValue := map[string]interface{}{
		"foo": make(chan int),
	}
	copiedValue := copyUnknown(unknownValue)
	var expectedValue interface{}
	require.Equal(t, expectedValue, copiedValue)
}
