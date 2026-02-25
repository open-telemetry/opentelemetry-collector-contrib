// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tokenlen

import (
	"bufio"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTokenLenState_Func(t *testing.T) {
	cases := []struct {
		name          string
		input         []byte
		atEOF         bool
		expectedLen   int
		expectedToken []byte
		expectedAdv   int
		expectedErr   error
	}{
		{
			name:        "no token yet",
			input:       []byte("partial"),
			atEOF:       false,
			expectedLen: len("partial"),
		},
		{
			name:          "complete token",
			input:         []byte("complete\ntoken"),
			atEOF:         false,
			expectedLen:   0, // should clear state after finding token
			expectedToken: []byte("complete"),
			expectedAdv:   len("complete\n"),
		},
		{
			name:        "growing token",
			input:       []byte("growing"),
			atEOF:       false,
			expectedLen: len("growing"),
		},
		{
			name:          "flush at EOF",
			input:         []byte("flush"),
			atEOF:         true,
			expectedLen:   0, // should clear state after flushing
			expectedToken: []byte("flush"),
			expectedAdv:   len("flush"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			state := &State{}
			splitFunc := state.Func(bufio.ScanLines)

			adv, token, err := splitFunc(tc.input, tc.atEOF)
			require.Equal(t, tc.expectedErr, err)
			require.Equal(t, tc.expectedToken, token)
			require.Equal(t, tc.expectedAdv, adv)
			require.Equal(t, tc.expectedLen, state.MinimumLength)
		})
	}
}

func TestTokenLenState_GrowingToken(t *testing.T) {
	state := &State{}
	splitFunc := state.Func(bufio.ScanLines)

	// First call with partial token
	adv, token, err := splitFunc([]byte("part"), false)
	require.NoError(t, err)
	require.Nil(t, token)
	require.Equal(t, 0, adv)
	require.Equal(t, len("part"), state.MinimumLength)

	// Second call with longer partial token
	adv, token, err = splitFunc([]byte("partial"), false)
	require.NoError(t, err)
	require.Nil(t, token)
	require.Equal(t, 0, adv)
	require.Equal(t, len("partial"), state.MinimumLength)

	// Final call with complete token
	adv, token, err = splitFunc([]byte("partial\ntoken"), false)
	require.NoError(t, err)
	require.Equal(t, []byte("partial"), token)
	require.Equal(t, len("partial\n"), adv)
	require.Equal(t, 0, state.MinimumLength) // State should be cleared after emitting token
}
