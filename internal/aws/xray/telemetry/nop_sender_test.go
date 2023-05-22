// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package telemetry

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNopRecorder(t *testing.T) {
	assert.Same(t, nopSenderInstance, NewNopSender())
	recorder := NewNopSender()
	assert.NotPanics(t, func() {
		recorder.Start()
		recorder.RecordConnectionError(nil)
		recorder.RecordSegmentsSent(1)
		recorder.RecordSegmentsSpillover(1)
		recorder.RecordSegmentsRejected(1)
		recorder.RecordSegmentsReceived(1)
		assert.False(t, recorder.HasRecording())
		assert.Nil(t, recorder.Rotate())
		recorder.Stop()
	})
}
