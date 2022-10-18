//go:build windows
// +build windows

package win_perf_counters

import (
	"testing"
	"unicode/utf16"

	"github.com/stretchr/testify/require"
)

func TestUTF16PtrToString(t *testing.T) {
	testPtr := (*uint16)(&utf16.Encode([]rune("Hello World\000"))[0])
	strOut := UTF16PtrToString(testPtr)
	require.Equal(t, "Hello World", strOut)
}

func TestUTF16PtrToString_nil(t *testing.T) {
	strOut := UTF16PtrToString(nil)
	require.Equal(t, "", strOut)
}
