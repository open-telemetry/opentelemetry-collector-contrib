//go:build windows
// +build windows

package win_perf_counters

import (
	"testing"
	"unicode/utf16"

	"github.com/stretchr/testify/require"
)

func TestUTF16PtrToString(t *testing.T) {
	t.Run("String 'Hello World'", func(t *testing.T) {
		testPtr := (*uint16)(&utf16.Encode([]rune("Hello World\000"))[0])
		strOut := UTF16PtrToString(testPtr)
		require.Equal(t, "Hello World", strOut)
	})

	t.Run("nil pointer", func(t *testing.T) {
		strOut := UTF16PtrToString(nil)
		require.Equal(t, "", strOut)
	})
}

func TestUTF16ToStringArray(t *testing.T) {
	testStr := "First String\000Second String\000Final String\000\000"
	testStrUTF16 := utf16.Encode([]rune(testStr))

	strs := UTF16ToStringArray(testStrUTF16)
	require.Equal(t, []string{
		"First String",
		"Second String",
		"Final String",
	}, strs)
}
