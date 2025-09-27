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
	var value any
	require.Nil(t, copyValue(value))
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
	value := []any{"test", true, 5}
	require.Equal(t, value, copyValue(value))
}

func TestCopyValueStringMap(t *testing.T) {
	value := map[string]string{"test": "value"}
	require.Equal(t, value, copyValue(value))
}

func TestCopyValueInterfaceMap(t *testing.T) {
	value := map[string]any{"test": 5}
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
	expectedValue := map[string]any{
		"Test": "value",
	}
	require.Equal(t, expectedValue, copiedValue)
}

func TestCopyAnyMap(t *testing.T) {
	t.Run("NonNil", func(t *testing.T) {
		require.Nil(t, copyAnyMap(nil))
	})

	t.Run("NonNil", func(t *testing.T) {
		stringMap := map[string]any{
			"message": "test",
		}
		copiedMap := copyAnyMap(stringMap)
		delete(stringMap, "message")
		require.Equal(t, "test", copiedMap["message"])
	})
}

func TestCopyAnySlice(t *testing.T) {
	t.Run("Nil", func(t *testing.T) {
		require.Nil(t, copyAnySlice(nil))
	})

	t.Run("NonNil", func(t *testing.T) {
		interfaceArray := []any{"test", 0, true}
		copiedArray := copyAnySlice(interfaceArray)
		interfaceArray[0] = "new"
		require.Equal(t, []any{"test", 0, true}, copiedArray)
	})
}

func TestCopyUnknownValueValid(t *testing.T) {
	type testStruct struct {
		Test string
	}
	unknownValue := testStruct{
		Test: "value",
	}
	copiedValue := copyUnknown(unknownValue)
	expectedValue := map[string]any{
		"Test": "value",
	}
	require.Equal(t, expectedValue, copiedValue)
}

func TestCopyUnknownValueInalid(t *testing.T) {
	unknownValue := map[string]any{
		"foo": make(chan int),
	}
	copiedValue := copyUnknown(unknownValue)
	var expectedValue any
	require.Equal(t, expectedValue, copiedValue)
}
