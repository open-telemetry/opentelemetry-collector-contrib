// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSelectPipeline(t *testing.T) {
	constants := PSConstants{
		RetryInterval: 50 * time.Millisecond,
		RetryGap:      10 * time.Millisecond,
		MaxRetries:    1000,
	}
	pS := NewPipelineSelector(5, constants, make(chan struct{}))

	idx, ch := pS.SelectedPipeline()

	require.Equal(t, idx, 0)
	require.Equal(t, pS.ChannelIndex(ch), 0)
}

func TestHandlePipelineError(t *testing.T) {

	done := make(chan struct{})

	constants := PSConstants{
		RetryInterval: 50 * time.Millisecond,
		RetryGap:      10 * time.Millisecond,
		MaxRetries:    1000,
	}
	pS := NewPipelineSelector(5, constants, done)

	go pS.ListenToChannels(done)

	idx, ch := pS.SelectedPipeline()

	require.Equal(t, idx, 0)

	ch <- false

	require.Eventually(t, func() bool {
		idx, _ = pS.SelectedPipeline()
		return idx == 1
	}, 3*time.Minute, 10*time.Millisecond)

	close(done)

}

func TestCurrentPipelineWithRetry(t *testing.T) {

	done := make(chan struct{})

	constants := PSConstants{
		RetryInterval: 50 * time.Millisecond,
		RetryGap:      10 * time.Millisecond,
		MaxRetries:    1000,
	}
	pS := NewPipelineSelector(5, constants, done)

	go pS.ListenToChannels(done)

	_, ch := pS.SelectedPipeline()

	ch <- false

	require.Eventually(t, func() bool {
		idx, _ := pS.SelectedPipeline()
		return idx == 0
	}, 3*time.Second, 5*time.Millisecond)

	require.Eventually(t, func() bool {
		idx, _ := pS.SelectedPipeline()
		return idx == 1
	}, 3*time.Second, 5*time.Millisecond)
	//
	require.Eventually(t, func() bool {
		idx, _ := pS.SelectedPipeline()
		return idx == 0
	}, 3*time.Second, 5*time.Millisecond)

	close(done)
}
