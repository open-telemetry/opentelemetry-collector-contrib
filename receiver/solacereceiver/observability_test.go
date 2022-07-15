// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package solacereceiver

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

type metricsTestCase struct {
	fn       func()        // function to test updating metrics
	v        *view.View    // view to reference
	m        stats.Measure // expected measure of the view
	calls    int           // number of times to call fn
	expected int           // expected value of reported metric at end of calls
}

func TestRecordMetrics(t *testing.T) {
	testCases := []metricsTestCase{
		{recordFailedReconnection, viewFailedReconnections, failedReconnections, 3, 3},
		{recordRecoverableUnmarshallingError, viewRecoverableUnmarshallingErrors, recoverableUnmarshallingErrors, 3, 3},
		{recordFatalUnmarshallingError, viewFatalUnmarshallingErrors, fatalUnmarshallingErrors, 3, 3},
		{recordDroppedSpanMessages, viewDroppedSpanMessages, droppedSpanMessages, 3, 3},
		{recordReceivedSpanMessages, viewReceivedSpanMessages, receivedSpanMessages, 3, 3},
		{recordReportedSpans, viewReportedSpans, reportedSpans, 3, 3},
		{func() {
			recordReceiverStatus(receiverStateTerminated)
		}, viewReceiverStatus, receiverStatus, 3, int(receiverStateTerminated)},
		{recordNeedUpgrade, viewNeedUpgrade, needUpgrade, 3, 1},
	}
	for _, tc := range testCases {
		t.Run(tc.m.Name(), func(t *testing.T) {
			for i := 0; i < tc.calls; i++ {
				tc.fn()
			}
			validateMetric(t, tc.v, tc.expected)
		})
	}
}

func validateMetric(t *testing.T, v *view.View, expected interface{}) {
	// hack to reset stats to 0
	defer func() {
		view.Unregister(v)
		err := view.Register(v)
		assert.NoError(t, err)
	}()
	rows, err := view.RetrieveData(v.Name)
	assert.NoError(t, err)
	if expected != nil {
		require.Len(t, rows, 1)
		value := reflect.Indirect(reflect.ValueOf(rows[0].Data)).FieldByName("Value").Interface()
		assert.EqualValues(t, expected, value)
	} else {
		assert.Len(t, rows, 0)
	}
}

// TestRegisterViewsExpectingPanic validates that if an error is returned from view.Register, we panic and don't continue with initialization
func TestRegisterViewsExpectingPanic(t *testing.T) {
	droppedSpanMessagesName := viewDroppedSpanMessages.Name
	defer func() {
		viewDroppedSpanMessages.Name = droppedSpanMessagesName
		registerMetrics()
	}()
	defer func() {
		r := recover()
		assert.NotNil(t, r)
	}()
	viewDroppedSpanMessages.Name = viewReceivedSpanMessages.Name
	registerMetrics()
}
