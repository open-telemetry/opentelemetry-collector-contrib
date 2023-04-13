// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
