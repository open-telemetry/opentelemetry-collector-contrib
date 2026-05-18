// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package msgpackvalidator

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/tinylib/msgp/msgp"
)

func TestValidateAcceptsBoundedPayload(t *testing.T) {
	var payload []byte
	payload = msgp.AppendMapHeader(payload, 1)
	payload = msgp.AppendString(payload, "stats")
	payload = msgp.AppendArrayHeader(payload, 2)
	payload = msgp.AppendNil(payload)
	payload = msgp.AppendString(payload, "ok")

	if err := Validate(payload); err != nil {
		t.Fatalf("Validate() returned unexpected error: %v", err)
	}
}

func TestValidateRejectsForgedHeaders(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
		want    string
	}{
		{
			name:    "array elements",
			payload: msgp.AppendArrayHeader(nil, maxMsgpackContainerElements+1),
			want:    "array length",
		},
		{
			name:    "array larger than remaining payload",
			payload: msgp.AppendArrayHeader(nil, 3),
			want:    "remaining payload",
		},
		{
			name:    "string bytes",
			payload: appendString32Header(maxMsgpackBinStringLen + 1),
			want:    "string length",
		},
		{
			name:    "binary bytes",
			payload: msgp.AppendBytesHeader(nil, maxMsgpackBinStringLen+1),
			want:    "bin length",
		},
		{
			name:    "extension bytes",
			payload: appendExt32Header(maxMsgpackExtLen + 1),
			want:    "extension length",
		},
		{
			name:    "nesting depth",
			payload: nestedArrayPayload(maxMsgpackDepth + 2),
			want:    "nesting exceeds depth limit",
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

func appendString32Header(size uint32) []byte {
	var sizeBytes [4]byte
	binary.BigEndian.PutUint32(sizeBytes[:], size)
	return append([]byte{0xdb}, sizeBytes[:]...)
}

func appendExt32Header(size uint32) []byte {
	var sizeBytes [4]byte
	binary.BigEndian.PutUint32(sizeBytes[:], size)
	payload := append([]byte{0xc9}, sizeBytes[:]...)
	return append(payload, 0)
}

func nestedArrayPayload(depth int) []byte {
	var payload []byte
	for range depth {
		payload = msgp.AppendArrayHeader(payload, 1)
	}
	return msgp.AppendNil(payload)
}
