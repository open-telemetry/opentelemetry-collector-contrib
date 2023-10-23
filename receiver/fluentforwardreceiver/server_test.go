// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package fluentforwardreceiver

import (
	"bufio"
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tinylib/msgp/msgp"
)

func TestDetermineNextEventMode(t *testing.T) {
	cases := []struct {
		name          string
		event         func() []byte
		expectedMode  EventMode
		expectedError error
	}{
		{
			"basic",
			func() []byte {
				var b []byte

				b = msgp.AppendArrayHeader(b, 3)
				b = msgp.AppendString(b, "my-tag")
				b = msgp.AppendInt(b, 5000)
				return b
			},
			MessageMode,
			nil,
		},
		{
			"str8-tag",
			func() []byte {
				var b []byte

				b = msgp.AppendArrayHeader(b, 3)
				b = msgp.AppendString(b, strings.Repeat("a", 128))
				b = msgp.AppendInt(b, 5000)
				return b
			},
			MessageMode,
			nil,
		},
		{
			"str16-tag",
			func() []byte {
				var b []byte

				b = msgp.AppendArrayHeader(b, 3)
				b = msgp.AppendString(b, strings.Repeat("a", 1024))
				b = msgp.AppendInt(b, 5000)
				return b
			},
			MessageMode,
			nil,
		},
		{
			"str32-tag",
			func() []byte {
				var b []byte

				b = msgp.AppendArrayHeader(b, 3)
				b = msgp.AppendString(b, strings.Repeat("a", 66000))
				b = msgp.AppendInt(b, 5000)
				return b
			},
			MessageMode,
			nil,
		},
		{
			"non-string-tag",
			func() []byte {
				var b []byte

				b = msgp.AppendArrayHeader(b, 3)
				b = msgp.AppendInt(b, 10)
				b = msgp.AppendInt(b, 5000)
				return b
			},
			UnknownMode,
			errors.New("malformed tag field"),
		},
		{
			"float-second-elm",
			func() []byte {
				var b []byte

				b = msgp.AppendArrayHeader(b, 3)
				b = msgp.AppendString(b, "my-tag")
				b = msgp.AppendFloat64(b, 5000.0)
				return b
			},
			UnknownMode,
			errors.New("unable to determine next event mode for type float64"),
		},
	}

	for i := range cases {
		c := cases[i]
		t.Run(c.name, func(t *testing.T) {
			peeker := bufio.NewReaderSize(bytes.NewReader(c.event()), 1024*100)
			mode, err := determineNextEventMode(peeker)
			if c.expectedError != nil {
				require.Equal(t, c.expectedError, err)
			} else {
				require.Equal(t, c.expectedMode, mode)
			}
		})
	}
}
