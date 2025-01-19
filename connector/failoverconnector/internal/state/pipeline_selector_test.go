// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"context"
	"sync"
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
	pS := NewPipelineSelector(5, constants)

	idx, ch := pS.SelectedPipeline()

	require.Equal(t, 0, idx)
	require.Equal(t, 0, pS.ChannelIndex(ch))
}

func TestHandlePipelineError(t *testing.T) {
	var wg sync.WaitGroup
	done := make(chan struct{})
	constants := PSConstants{
		RetryInterval: 50 * time.Millisecond,
		RetryGap:      10 * time.Millisecond,
		MaxRetries:    1000,
	}
	pS := NewPipelineSelector(5, constants)

	wg.Add(1)
	go pS.ListenToChannels(done, &wg)
	defer func() {
		close(done)
		wg.Wait()
	}()

	idx, ch := pS.SelectedPipeline()
	require.Equal(t, 0, idx)
	ch <- false

	require.Eventually(t, func() bool {
		idx, _ = pS.SelectedPipeline()
		return idx == 1
	}, 3*time.Minute, 5*time.Millisecond)
}

func TestCurrentPipelineWithRetry(t *testing.T) {
	constants := PSConstants{
		RetryInterval: 50 * time.Millisecond,
		RetryGap:      10 * time.Millisecond,
		MaxRetries:    1000,
	}
	pS := NewPipelineSelector(5, constants)

	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
	}()

	pS.TestSetStableIndex(2)
	pS.TestRetryPipelines(ctx, constants.RetryInterval, constants.RetryGap)

	require.Eventually(t, func() bool {
		idx, _ := pS.SelectedPipeline()
		return idx == 0
	}, 3*time.Second, 5*time.Millisecond)

	require.Eventually(t, func() bool {
		idx, _ := pS.SelectedPipeline()
		return idx == 1
	}, 3*time.Second, 5*time.Millisecond)

	require.Eventually(t, func() bool {
		idx, _ := pS.SelectedPipeline()
		return idx == 0
	}, 3*time.Second, 5*time.Millisecond)
}
