// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package xconfigunit

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestByteSizeUnmarshalText(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    ByteSize
		wantErr bool
	}{
		{name: "bytes", input: "12B", want: 12},
		{name: "kilobytes", input: "3KB", want: 3 * KiloByteUnit},
		{name: "megabytes", input: "4MB", want: 4 * MegaByteUnit},
		{name: "gigabytes", input: "2GB", want: 2 * GigaByteUnit},
		{name: "compound", input: "100MB20KB", want: 100*MegaByteUnit + 20*KiloByteUnit},
		{name: "shorthand", input: "1G2M3K4B", want: GigaByteUnit + 2*MegaByteUnit + 3*KiloByteUnit + 4},
		{name: "empty", wantErr: true},
		{name: "unit without number", input: "MB", wantErr: true},
		{name: "number without unit", input: "10", wantErr: true},
		{name: "unknown unit", input: "10XB", wantErr: true},
		{name: "missing number", input: "1MBKB", wantErr: true},
		{name: "negative value", input: "-1MB", want: -MegaByteUnit},
		{name: "negative compound", input: "-100MB20KB", want: -100*MegaByteUnit - 20*KiloByteUnit},
		{name: "overflow", input: "9223372036855MB", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got ByteSize
			err := got.UnmarshalText([]byte(tt.input))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestByteSizeTextRoundTrip(t *testing.T) {
	for _, want := range []ByteSize{0, 1, 100020000, -100020000, GigaByteUnit + 2*MegaByteUnit + 3*KiloByteUnit + 4} {
		text, err := want.MarshalText()
		require.NoError(t, err)

		var got ByteSize
		require.NoError(t, got.UnmarshalText(text))
		require.Equal(t, want, got)
	}
}

func TestByteSizeString(t *testing.T) {
	tests := []struct {
		value ByteSize
		want  string
	}{
		{value: 0, want: "0B"},
		{value: 12, want: "12B"},
		{value: KiloByteUnit, want: "1KB"},
		{value: MegaByteUnit, want: "1MB"},
		{value: GigaByteUnit, want: "1GB"},
		{value: 100020000, want: "100MB20KB"},
		{value: -100020000, want: "-100MB20KB"},
	}

	for _, tt := range tests {
		require.Equal(t, tt.want, tt.value.String())
	}
}
