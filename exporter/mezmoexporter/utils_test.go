// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package mezmoexporter

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTruncateString(t *testing.T) {
	t.Run("Test empty string", func(t *testing.T) {
		s := truncateString("", 10)
		require.Len(t, s, 0)
	})

	// Test string is less than the maximum length
	t.Run("Test shorter string", func(t *testing.T) {
		s := truncateString("short", 10)
		require.Len(t, s, 5)
		require.Equal(t, s, "short")
	})

	// Test string is equal to the maximum length
	t.Run("Test equal string", func(t *testing.T) {
		s := truncateString("short", 5)
		require.Len(t, s, 5)
		require.Equal(t, s, "short")
	})

	// Test string is longer than the maximum length
	t.Run("Test longer string", func(t *testing.T) {
		s := truncateString("longstring", 4)
		require.Len(t, s, 4)
		require.Equal(t, s, "long")
	})
}

func TestRandString(t *testing.T) {
	t.Run("Test fixed length string", func(t *testing.T) {
		var s = randString(16 * 1024)
		require.Len(t, s, 16*1024)
	})
}

const letters = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// randString Returns a random string of the specified length.
func randString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
