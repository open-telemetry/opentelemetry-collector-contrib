package jaegerencodingextension

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestBufferInt(t *testing.T) {
	for n := 0; n < 1024*1024; n++ {
		buff, err := newWritableBuffer()
		require.NoError(t, err)
		require.NoError(t, buff.writeInt(n))
		bts := buff.bytes()

		readBuff := newReadableBuffer(bts)
		n0, err0 := readBuff.readInt()
		require.NoError(t, err0)
		require.Equal(t, n, n0)
	}
}

func TestBuffSlices(t *testing.T) {
	arr := make([]string, 0)
	for n := 0; n < 100; n++ {
		v := fmt.Sprintf("OpenTelemetry Collector %d", n)
		arr = append(arr, v)
	}
	writableBuff, err := newWritableBuffer()
	require.NoError(t, err)
	for _, s := range arr {
		require.NoError(t, writableBuff.writeBytes([]byte(s)))
	}
	bts := writableBuff.bytes()

	readBuff := newReadableBuffer(bts)
	arrs, err := readBuff.slices()
	require.NoError(t, err)
	require.Equal(t, len(arrs), 100)

	for i, e := range arrs {
		v := string(e)
		v0 := arr[i]
		require.Equal(t, v, v0)
	}
}

func TestReadOnlyBuffCompatibility(t *testing.T) {
	str := "OpenTelemetry Collector"
	buff := newReadableBuffer([]byte(str))
	slices, err := buff.slices()
	require.NoError(t, err)
	require.Equal(t, len(slices), 1)
	require.Equal(t, string(slices[0]), str)
}
