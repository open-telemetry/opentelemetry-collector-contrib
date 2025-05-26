// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package state

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSelectPipeline(t *testing.T) {
	done := make(chan struct{})
	retryChan := make(chan struct{}, 1)
	constants := PSConstants{
		RetryInterval: 50 * time.Millisecond,
	}
	pS := NewPipelineSelector(retryChan, done, constants)

	idx := pS.CurrentPipeline()

	require.Equal(t, 0, idx)
}

func TestHandlePipelineError(t *testing.T) {
	done := make(chan struct{})
	retryChan := make(chan struct{}, 1)
	constants := PSConstants{
		RetryInterval: 50 * time.Millisecond,
	}
	pS := NewPipelineSelector(retryChan, done, constants)

	defer func() {
		close(done)
	}()

	idx := pS.CurrentPipeline()
	require.Equal(t, 0, idx)

	pS.HandleError(0)

	require.Eventually(t, func() bool {
		idx = pS.CurrentPipeline()
		return idx == 1
	}, 3*time.Minute, 5*time.Millisecond)
}

func TestCurrentPipelineWithReset(t *testing.T) {
	done := make(chan struct{})
	retryChan := make(chan struct{}, 1)
	constants := PSConstants{
		RetryInterval: 50 * time.Millisecond,
	}
	pS := NewPipelineSelector(retryChan, done, constants)

	defer func() {
		close(done)
	}()

	pS.TestSetCurrentPipeline(2)
	pS.ResetHealthyPipeline(1)

	require.Eventually(t, func() bool {
		idx := pS.CurrentPipeline()
		return idx == 1
	}, 3*time.Second, 5*time.Millisecond)

	pS.ResetHealthyPipeline(0)

	require.Eventually(t, func() bool {
		idx := pS.CurrentPipeline()
		return idx == 0
	}, 3*time.Second, 5*time.Millisecond)
}
