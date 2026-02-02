// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package attraction

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestSha2Hasher(t *testing.T) {
	tests := []struct {
		name string
		in   pcommon.Value
		want string
	}{
		{
			name: "string",
			in:   pcommon.NewValueStr("foo"),
			want: sha256Hex(t, []byte("foo")),
		},
		{
			name: "bool_true",
			in:   pcommon.NewValueBool(true),
			want: sha256Hex(t, []byte{1}),
		},
		{
			name: "bool_false",
			in:   pcommon.NewValueBool(false),
			want: sha256Hex(t, []byte{0}),
		},
		{
			name: "int64",
			in:   pcommon.NewValueInt(24),
			want: func() string {
				var b [8]byte
				binary.LittleEndian.PutUint64(b[:], uint64(24))
				return sha256Hex(t, b[:])
			}(),
		},
		{
			name: "double",
			in:   pcommon.NewValueDouble(2.4),
			want: func() string {
				var b [8]byte
				binary.LittleEndian.PutUint64(b[:], math.Float64bits(2.4))
				return sha256Hex(t, b[:])
			}(),
		},
		{
			name: "unsupported_type_empty",
			in:   pcommon.NewValueEmpty(),
		},
		{
			name: "unsupported_type_bytes",
			in: func() pcommon.Value {
				v := pcommon.NewValueBytes()
				v.Bytes().Append(1, 2, 3)
				return v
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			sha2Hasher(tt.in)
			require.Equal(t, tt.want, tt.in.Str())
		})
	}
}

func sha256Hex(t *testing.T, b []byte) string {
	t.Helper()
	sum := sha256.Sum256(b)
	var out [sha256.Size * 2]byte
	hex.Encode(out[:], sum[:])
	return string(out[:])
}
