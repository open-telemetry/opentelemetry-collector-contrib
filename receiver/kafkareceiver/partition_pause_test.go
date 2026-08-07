// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package kafkareceiver

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPartitionPauseState(t *testing.T) {
	type action struct {
		pause  bool
		reason partitionPauseReason
	}
	cases := []struct {
		name            string
		actions         []action
		wantPauseCalls  int
		wantResumeCalls int
	}{
		{
			name: "resumes after the only reason clears",
			actions: []action{
				{
					pause:  true,
					reason: partitionPauseBackpressure,
				},
				{
					reason: partitionPauseBackpressure,
				},
			},
			wantPauseCalls:  1,
			wantResumeCalls: 1,
		},
		{
			name: "does not resume while processing error remains",
			actions: []action{
				{
					pause:  true,
					reason: partitionPauseBackpressure,
				},
				{
					pause:  true,
					reason: partitionPauseProcessingError,
				},
				{
					reason: partitionPauseBackpressure,
				},
			},
			wantPauseCalls: 1,
		},
		{
			name: "duplicate operations do not change fetch state",
			actions: []action{
				{
					pause:  true,
					reason: partitionPauseRewind,
				},
				{
					pause:  true,
					reason: partitionPauseRewind,
				},
				{
					reason: partitionPauseRewind,
				},
				{
					reason: partitionPauseRewind,
				},
			},
			wantPauseCalls:  1,
			wantResumeCalls: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			controller := &countingPartitionFetchController{}
			state := partitionPauseState{}
			partition := map[string][]int32{"topic": {0}}

			for _, action := range tc.actions {
				if action.pause {
					state.pause(controller, partition, action.reason)
				} else {
					state.resume(controller, partition, action.reason)
				}
			}

			require.Equal(t, tc.wantPauseCalls, controller.pauseCalls)
			require.Equal(t, tc.wantResumeCalls, controller.resumeCalls)
		})
	}
}

type countingPartitionFetchController struct {
	pauseCalls  int
	resumeCalls int
}

func (c *countingPartitionFetchController) PauseFetchPartitions(map[string][]int32) map[string][]int32 {
	c.pauseCalls++
	return nil
}

func (c *countingPartitionFetchController) ResumeFetchPartitions(map[string][]int32) {
	c.resumeCalls++
}
