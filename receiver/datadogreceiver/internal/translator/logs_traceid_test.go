// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package translator

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
)

func TestParseDatadogTraceID_HexVersusDecimal(t *testing.T) {
	mustTraceID := func(h string) pcommon.TraceID {
		b, err := hex.DecodeString(h)
		require.NoError(t, err)
		var id pcommon.TraceID
		copy(id[:], b)
		return id
	}

	// A 128-bit id with a realistic timestamp in the upper 32 bits, rendered as decimal.
	realID := mustTraceID("69000000000000000000000000000001")
	decimal := new(big.Int).SetBytes(realID[:]).String()
	require.NotEqual(t, 32, len(decimal), "test setup: decimal form must not be 32 chars")

	tests := []struct {
		name string
		in   string
		want pcommon.TraceID
	}{
		{
			name: "all-digit 32-char hex is parsed as hex",
			in:   "12345678901234567890123456789012",
			want: mustTraceID("12345678901234567890123456789012"),
		},
		{
			name: "32-char hex with letters is parsed as hex",
			in:   "6553f1000000000000000000000000ab",
			want: mustTraceID("6553f1000000000000000000000000ab"),
		},
		{
			name: "128-bit decimal is still parsed as decimal",
			in:   decimal,
			want: realID,
		},
		{
			name: "64-bit decimal goes to the low 64 bits",
			in:   "1234567890",
			want: mustTraceID("000000000000000000000000499602d2"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parseDatadogTraceID(tt.in, "")
			require.True(t, ok)
			assert.Equal(t, tt.want, got)
		})
	}
}
