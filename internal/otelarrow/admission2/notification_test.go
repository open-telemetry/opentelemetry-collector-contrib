// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package admission2

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNotification(t *testing.T) {
	require := require.New(t)

	start := newNotification()
	require.False(start.HasBeenNotified())

	done := make([]N, 5)
	for i := range 5 {
		done[i] = newNotification()
		go func(i int) {
			start.WaitForNotification()
			done[i].Notify()
		}(i)
	}

	// None of the goroutines can make progress until we notify
	// start.
	for now := time.Now(); time.Now().Before(now.Add(100 * time.Millisecond)); {
		for _, n := range done {
			require.False(n.HasBeenNotified())
		}
	}

	start.Notify()

	// Now the goroutines can finish.
	for _, n := range done {
		n.WaitForNotification()
		require.True(n.HasBeenNotified())
	}
}
