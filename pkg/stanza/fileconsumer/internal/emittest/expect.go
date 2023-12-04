// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package emittest

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func WaitForEmit(t *testing.T, c chan *Call) *Call {
	select {
	case call := <-c:
		return call
	case <-time.After(3 * time.Second):
		require.FailNow(t, "Timed out waiting for message")
		return nil
	}
}

func WaitForToken(t *testing.T, c chan *Call, expected []byte) {
	select {
	case call := <-c:
		require.Equal(t, expected, call.Token)
	case <-time.After(3 * time.Second):
		require.FailNow(t, fmt.Sprintf("Timed out waiting for token: %s", expected))
	}
}

func WaitForTokens(t *testing.T, c chan *Call, expected ...[]byte) {
	actual := make([][]byte, 0, len(expected))
LOOP:
	for {
		select {
		case call := <-c:
			actual = append(actual, call.Token)
		case <-time.After(3 * time.Second):
			break LOOP
		}
	}

	require.ElementsMatch(t, expected, actual)
}

func WaitForN(t *testing.T, c chan *Call, n int) [][]byte {
	emitChan := make([][]byte, 0, n)
	for i := 0; i < n; i++ {
		select {
		case call := <-c:
			emitChan = append(emitChan, call.Token)
		case <-time.After(3 * time.Second):
			require.FailNow(t, "Timed out waiting for message")
			return nil
		}
	}
	return emitChan
}

func WaitForCall(t *testing.T, c chan *Call, expected []byte, attrs map[string]any) {
	select {
	case call := <-c:
		require.Equal(t, expected, call.Token)
		require.Equal(t, attrs, call.Attributes)
	case <-time.After(3 * time.Second):
		require.FailNow(t, fmt.Sprintf("Timed out waiting for token: %s", expected))
	}
}

// TODO maybe remove this?
func ExpectNoTokens(t *testing.T, c chan *Call) {
	ExpectNoTokensUntil(t, c, 200*time.Millisecond)
}

func ExpectNoTokensUntil(t *testing.T, c chan *Call, d time.Duration) {
	select {
	case token := <-c:
		require.FailNow(t, "Received unexpected message", "Message: %s", token)
	case <-time.After(d):
	}
}
