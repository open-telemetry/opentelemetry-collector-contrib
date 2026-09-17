// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package serializeprofiles

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/ebpf-profiler/libpf"
)

const (
	fileIDLo      = 0x77efa716a912a492
	fileIDHi      = 0x17445787329fd29a
	addressOrLine = 0xe51c
)

func TestFrameID(t *testing.T) {
	fileID := libpf.NewFileID(fileIDLo, fileIDHi)

	tests := []struct {
		name     string
		input    string
		expected frameID
		bytes    []byte
		err      error
	}{
		{
			name:     "frame base64",
			input:    "d--nFqkSpJIXRFeHMp_SmgAAAAAAAOUc",
			expected: newFrameID(fileID, addressOrLine),
			bytes: []byte{
				0x77, 0xef, 0xa7, 0x16, 0xa9, 0x12, 0xa4, 0x92, 0x17, 0x44, 0x57, 0x87,
				0x32, 0x9f, 0xd2, 0x9a, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xe5, 0x1c,
			},
			err: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fID, err := newFrameIDFromString(test.input)
			assert.Equal(t, test.err, err)
			assert.Equal(t, test.expected, fID)

			// check if the roundtrip back to the input works
			assert.Equal(t, test.input, fID.String())

			assert.Equal(t, test.bytes, fID.Bytes())

			fID, err = newFrameIDFromBytes(fID.Bytes())
			assert.Equal(t, test.err, err)
			assert.Equal(t, test.expected, fID)
		})
	}
}
