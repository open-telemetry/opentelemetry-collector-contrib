// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecode(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		messages    []string
		wantDecoded [][]byte
		wantErr     error
	}{
		"TestMessagesWithDelimiter": {
			messages: []string{
				base64.StdEncoding.EncodeToString([]byte("Message one")),
				base64.StdEncoding.EncodeToString([]byte("Message two after delimiter")),
				base64.StdEncoding.EncodeToString([]byte("Message three")),
			},
			wantDecoded: [][]byte{
				[]byte("Message one"),
				[]byte("Message two after delimiter"),
				[]byte("Message three"),
			},
		},
		"TestInvalidBase64": {
			messages: []string{
				"Invalid message",
			},
			wantDecoded: nil,
			wantErr:     base64.CorruptInputError(7),
		},
		"TestEmptyMessages": {
			messages:    []string{},
			wantDecoded: [][]byte{}, // Decode returns empty slice, not nil
		},
	}

	decoder := NewBinaryDecoder()
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			rawMessages, err := json.Marshal(test.messages)
			require.NoError(t, err)

			decodedMessages, err := decoder.Decode(string(rawMessages))
			if test.wantErr != nil {
				require.Error(t, err)
				assert.True(t, errors.Is(err, test.wantErr), "err should wrap %v", test.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.wantDecoded, decodedMessages)
		})
	}
}
