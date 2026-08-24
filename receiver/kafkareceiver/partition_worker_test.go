// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestClearPauseReasons proves fetch resumes only after the last pause reason is cleared.
func TestClearPauseReasons(t *testing.T) {
	cases := []struct {
		name          string
		current       partitionPauseReason
		clear         partitionPauseReason
		wantResume    bool
		wantRemaining partitionPauseReason
	}{
		{
			name:  "does not resume without matching reason",
			clear: partitionPauseBackpressure,
		},
		{
			name:          "resumes after final reason clears",
			current:       partitionPauseBackpressure,
			clear:         partitionPauseBackpressure,
			wantResume:    true,
			wantRemaining: 0,
		},
		{
			name:          "does not resume while another reason remains",
			current:       partitionPauseBackpressure | partitionPauseRewind,
			clear:         partitionPauseBackpressure,
			wantRemaining: partitionPauseRewind,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			partitionConsumer := &pc{}
			partitionConsumer.pauseReasons.Store(uint32(tc.current))

			require.Equal(t, tc.wantResume, partitionConsumer.clearPauseReasons(tc.clear))
			require.Equal(t, uint32(tc.wantRemaining), partitionConsumer.pauseReasons.Load())
		})
	}
}
