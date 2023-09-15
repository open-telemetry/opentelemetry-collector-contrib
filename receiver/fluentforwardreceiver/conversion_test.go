// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardreceiver

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tinylib/msgp/msgp"
	"go.opentelemetry.io/collector/pdata/pcommon"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/plogtest"
)

func TestMessageEventConversion(t *testing.T) {
	eventBytes := parseHexDump("testdata/message-event")
	reader := msgp.NewReader(bytes.NewReader(eventBytes))

	var event MessageEventLogRecord
	err := event.DecodeMsg(reader)
	require.Nil(t, err)

	expectedLog := Logs(
		Log{
			Timestamp: 1593031012000000000,
			Body:      pcommon.NewValueStr("..."),
			Attributes: map[string]interface{}{
				"container_id":   "b00a67eb645849d6ab38ff8beb4aad035cc7e917bf123c3e9057c7e89fc73d2d",
				"container_name": "/unruffled_cannon",
				"fluent.tag":     "b00a67eb6458",
				"source":         "stdout",
			},
		},
	).ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0)
	require.NoError(t, plogtest.CompareLogRecord(expectedLog, event.LogRecords().At(0)))
}

func TestAttributeTypeConversion(t *testing.T) {
	var b []byte

	b = msgp.AppendArrayHeader(b, 3)
	b = msgp.AppendString(b, "my-tag")
	b = msgp.AppendInt(b, 5000)
	b = msgp.AppendMapHeader(b, 16)
	b = msgp.AppendString(b, "a")
	b = msgp.AppendFloat64(b, 5.0)
	b = msgp.AppendString(b, "b")
	b = msgp.AppendFloat32(b, 6.0)
	b = msgp.AppendString(b, "c")
	b = msgp.AppendBool(b, true)
	b = msgp.AppendString(b, "d")
	b = msgp.AppendInt8(b, 1)
	b = msgp.AppendString(b, "e")
	b = msgp.AppendInt16(b, 2)
	b = msgp.AppendString(b, "f")
	b = msgp.AppendInt32(b, 3)
	b = msgp.AppendString(b, "g")
	b = msgp.AppendInt64(b, 4)
	b = msgp.AppendString(b, "h")
	b = msgp.AppendUint8(b, ^uint8(0))
	b = msgp.AppendString(b, "i")
	b = msgp.AppendUint16(b, ^uint16(0))
	b = msgp.AppendString(b, "j")
	b = msgp.AppendUint32(b, ^uint32(0))
	b = msgp.AppendString(b, "k")
	b = msgp.AppendUint64(b, ^uint64(0))
	b = msgp.AppendString(b, "l")
	b = msgp.AppendComplex64(b, complex64(0))
	b = msgp.AppendString(b, "m")
	b = msgp.AppendBytes(b, []byte{0x1, 0x65, 0x2})
	b = msgp.AppendString(b, "n")
	b = msgp.AppendArrayHeader(b, 2)
	b = msgp.AppendString(b, "first")
	b = msgp.AppendString(b, "second")
	b = msgp.AppendString(b, "o")
	b, err := msgp.AppendIntf(b, []uint8{99, 100, 101})
	b = msgp.AppendString(b, "p")
	b = msgp.AppendNil(b)

	require.NoError(t, err)

	reader := msgp.NewReader(bytes.NewReader(b))

	var event MessageEventLogRecord
	err = event.DecodeMsg(reader)
	require.Nil(t, err)

	le := event.LogRecords().At(0)

	require.NoError(t, plogtest.CompareLogRecord(Logs(
		Log{
			Timestamp: 5000000000000,
			Body:      pcommon.NewValueEmpty(),
			Attributes: map[string]interface{}{
				"a":          5.0,
				"b":          6.0,
				"c":          true,
				"d":          1,
				"e":          2,
				"f":          3,
				"fluent.tag": "my-tag",
				"g":          4,
				"h":          255,
				"i":          65535,
				"j":          4294967295,
				"k":          -1,
				"l":          "(0+0i)",
				"m":          "\001e\002",
				"n":          []interface{}{"first", "second"},
				"o":          "cde",
				"p":          nil,
			},
		},
	).ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0), le))
}

func TestEventMode(t *testing.T) {
	require.Equal(t, "unknown", UnknownMode.String())
	require.Equal(t, "message", MessageMode.String())
	require.Equal(t, "forward", ForwardMode.String())
	require.Equal(t, "packedforward", PackedForwardMode.String())

	const TestMode EventMode = 6
	require.Panics(t, func() { _ = TestMode.String() })
}

func TestTimeFromTimestampBadType(t *testing.T) {
	_, err := timeFromTimestamp("bad")
	require.NotNil(t, err)
}

func TestMessageEventConversionWithErrors(t *testing.T) {
	var b []byte

	b = msgp.AppendArrayHeader(b, 3)
	b = msgp.AppendString(b, "my-tag")
	b = msgp.AppendInt(b, 5000)
	b = msgp.AppendMapHeader(b, 1)
	b = msgp.AppendString(b, "a")
	b = msgp.AppendFloat64(b, 5.0)

	for i := 0; i < len(b)-1; i++ {
		t.Run(fmt.Sprintf("EOF at byte %d", i), func(t *testing.T) {
			reader := msgp.NewReader(bytes.NewReader(b[:i]))

			var event MessageEventLogRecord
			err := event.DecodeMsg(reader)
			require.NotNil(t, err)
		})
	}
}

func TestForwardEventConversionWithErrors(t *testing.T) {
	b := parseHexDump("testdata/forward-event")

	for i := 0; i < len(b)-1; i++ {
		t.Run(fmt.Sprintf("EOF at byte %d", i), func(t *testing.T) {
			reader := msgp.NewReader(bytes.NewReader(b[:i]))

			var event ForwardEventLogRecords
			err := event.DecodeMsg(reader)
			require.NotNil(t, err)
		})
	}
}

func TestPackedForwardEventConversionWithErrors(t *testing.T) {
	b := parseHexDump("testdata/forward-packed-compressed")

	for i := 0; i < len(b)-1; i++ {
		t.Run(fmt.Sprintf("EOF at byte %d", i), func(t *testing.T) {
			reader := msgp.NewReader(bytes.NewReader(b[:i]))

			var event PackedForwardEventLogRecords
			err := event.DecodeMsg(reader)
			require.NotNil(t, err)
		})
	}

	t.Run("Invalid gzip header", func(t *testing.T) {
		in := make([]byte, len(b))
		copy(in, b)
		in[0x71] = 0xff
		reader := msgp.NewReader(bytes.NewReader(in))

		var event PackedForwardEventLogRecords
		err := event.DecodeMsg(reader)
		require.NotNil(t, err)
		require.Contains(t, err.Error(), "gzip")
		fmt.Println(err.Error())
	})
}

func TestBodyConversion(t *testing.T) {
	var b []byte

	b = msgp.AppendArrayHeader(b, 3)
	b = msgp.AppendString(b, "my-tag")
	b = msgp.AppendInt(b, 5000)
	b = msgp.AppendMapHeader(b, 1)
	b = msgp.AppendString(b, "log")
	b = msgp.AppendMapHeader(b, 3)
	b = msgp.AppendString(b, "a")
	b = msgp.AppendString(b, "value")
	b = msgp.AppendString(b, "b")
	b = msgp.AppendArrayHeader(b, 2)
	b = msgp.AppendString(b, "first")
	b = msgp.AppendString(b, "second")
	b = msgp.AppendString(b, "c")
	b = msgp.AppendMapHeader(b, 1)
	b = msgp.AppendString(b, "d")
	b = msgp.AppendInt(b, 24)
	b = msgp.AppendString(b, "o")
	b, err := msgp.AppendIntf(b, []uint8{99, 100, 101})

	require.NoError(t, err)

	reader := msgp.NewReader(bytes.NewReader(b))

	var event MessageEventLogRecord
	err = event.DecodeMsg(reader)
	require.Nil(t, err)

	le := event.LogRecords().At(0)

	body := pcommon.NewValueMap()
	body.Map().PutStr("a", "value")

	bv := body.Map().PutEmptySlice("b")
	bv.AppendEmpty().SetStr("first")
	bv.AppendEmpty().SetStr("second")

	cv := body.Map().PutEmptyMap("c")
	cv.PutInt("d", 24)

	require.NoError(t, plogtest.CompareLogRecord(Logs(
		Log{
			Timestamp: 5000000000000,
			Body:      body,
			Attributes: map[string]interface{}{
				"fluent.tag": "my-tag",
			},
		},
	).ResourceLogs().At(0).ScopeLogs().At(0).LogRecords().At(0), le))
}
