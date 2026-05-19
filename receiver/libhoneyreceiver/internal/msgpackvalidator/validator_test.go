// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package msgpackvalidator

import (
	"encoding/binary"
	"strings"
	"testing"
)

func TestValidateAcceptsBoundedPayload(t *testing.T) {
	payload := []byte{
		0x81,
		0xa4, 'd', 'a', 't', 'a',
		0x81,
		0xa2, 'o', 'k',
		0xc3,
	}

	if err := Validate(payload); err != nil {
		t.Fatalf("Validate() returned unexpected error: %v", err)
	}
}

func TestValidateRejectsForgedLengthHeaders(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		want    string
	}{
		{
			name:    "array length over limit",
			payload: appendLength32Header(0xdd, maxMsgpackContainerElements+1),
			want:    "array length",
		},
		{
			name:    "array length exceeds payload",
			payload: []byte{0x93},
			want:    "remaining payload",
		},
		{
			name:    "map length over limit",
			payload: appendLength32Header(0xdf, maxMsgpackContainerElements+1),
			want:    "map length",
		},
		{
			name:    "string length over limit",
			payload: appendLength32Header(0xdb, maxMsgpackBinStringLen+1),
			want:    "string length",
		},
		{
			name:    "binary length over limit",
			payload: appendLength32Header(0xc6, maxMsgpackBinStringLen+1),
			want:    "bin length",
		},
		{
			name:    "extension length over limit",
			payload: append(appendLength32Header(0xc9, maxMsgpackBinStringLen+1), 0),
			want:    "extension length",
		},
		{
			name:    "trailing bytes",
			payload: []byte{0xc0, 0xc0},
			want:    "trailing bytes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.payload)
			if err == nil {
				t.Fatal("Validate() returned nil error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %q, want substring %q", err.Error(), tt.want)
			}
		})
	}
}

func appendLength32Header(prefix byte, size uint64) []byte {
	var sizeBytes [4]byte
	binary.BigEndian.PutUint32(sizeBytes[:], uint32(size))
	return append([]byte{prefix}, sizeBytes[:]...)
}
