// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package remotetapprocessor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestChannelsetShutdownWithNonContiguousKeys guards against shutdown panicking when
// the channelSet's keys are not a contiguous 0..n-1 range, which happens whenever any
// channel added earlier than others was already removed (e.g. a websocket client
// disconnected mid-stream before the processor shuts down).
func TestChannelsetShutdownWithNonContiguousKeys(t *testing.T) {
	cs := newChannelSet()
	ch0 := make(chan []byte, 1)
	ch1 := make(chan []byte, 1)
	key0 := cs.add(ch0)
	_ = cs.add(ch1)

	cs.closeAndRemove(key0)

	assert.NotPanics(t, func() {
		cs.shutdown()
	})

	select {
	case _, stillOpen := <-ch1:
		assert.False(t, stillOpen, "remaining channel should have been closed by shutdown")
	case <-time.After(time.Second):
		t.Error("timed out waiting for remaining channel to be closed by shutdown")
	}
}
