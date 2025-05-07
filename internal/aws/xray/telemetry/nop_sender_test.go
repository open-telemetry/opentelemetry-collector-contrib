// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNopRecorder(t *testing.T) {
	assert.Same(t, nopSenderInstance, NewNopSender())
	recorder := NewNopSender()
	assert.NotPanics(t, func() {
		recorder.Start(context.Background())
		recorder.RecordConnectionError(nil)
		recorder.RecordSegmentsSent(1)
		recorder.RecordSegmentsSpillover(1)
		recorder.RecordSegmentsRejected(1)
		recorder.RecordSegmentsReceived(1)
		assert.False(t, recorder.HasRecording())
		assert.Zero(t, recorder.Rotate())
		recorder.Stop()
	})
}
