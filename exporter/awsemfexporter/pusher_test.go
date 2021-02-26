// Copyright 2020, OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package awsemfexporter

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

//
//  LogEvent Tests
//
func TestLogEvent_eventPayloadBytes(t *testing.T) {
	testMessage := "test message"
	logEvent := NewLogEvent(0, testMessage)
	assert.Equal(t, len(testMessage)+PerEventHeaderBytes, logEvent.eventPayloadBytes())
}

//
//  LogEventBatch Tests
//
func TestLogEventBatch_timestampWithin24Hours(t *testing.T) {
	min := time.Date(2017, time.June, 20, 23, 38, 0, 0, time.Local)
	max := min.Add(23 * time.Hour)
	logEventBatch := &LogEventBatch{
		maxTimestampInMillis: max.UnixNano() / 1e6,
		minTimestampInMillis: min.UnixNano() / 1e6,
	}

	//less than the min
	target := min.Add(-1 * time.Hour)
	assert.True(t, logEventBatch.timestampWithin24Hours(aws.Int64(target.UnixNano()/1e6)))

	target = target.Add(-1 * time.Millisecond)
	assert.False(t, logEventBatch.timestampWithin24Hours(aws.Int64(target.UnixNano()/1e6)))

	//more than the max
	target = max.Add(1 * time.Hour)
	assert.True(t, logEventBatch.timestampWithin24Hours(aws.Int64(target.UnixNano()/1e6)))

	target = target.Add(1 * time.Millisecond)
	assert.False(t, logEventBatch.timestampWithin24Hours(aws.Int64(target.UnixNano()/1e6)))

	//in between min and max
	target = min.Add(2 * time.Hour)
	assert.True(t, logEventBatch.timestampWithin24Hours(aws.Int64(target.UnixNano()/1e6)))
}

func TestLogEventBatch_sortLogEvents(t *testing.T) {
	totalEvents := 10
	logEventBatch := &LogEventBatch{
		PutLogEventsInput: &cloudwatchlogs.PutLogEventsInput{
			LogEvents: make([]*cloudwatchlogs.InputLogEvent, 0, totalEvents)}}

	for i := 0; i < totalEvents; i++ {
		timestamp := rand.Int()
		logEvent := NewLogEvent(
			int64(timestamp),
			fmt.Sprintf("message%v", timestamp))
		fmt.Printf("logEvents[%d].Timestamp=%d.\n", i, timestamp)
		logEventBatch.PutLogEventsInput.LogEvents = append(logEventBatch.PutLogEventsInput.LogEvents, logEvent.InputLogEvent)
	}

	logEventBatch.sortLogEvents()

	logEvents := logEventBatch.PutLogEventsInput.LogEvents
	for i := 1; i < totalEvents; i++ {
		fmt.Printf("logEvents[%d].Timestamp=%d, logEvents[%d].Timestamp=%d.\n", i-1, *logEvents[i-1].Timestamp, i, *logEvents[i].Timestamp)
		assert.True(t, *logEvents[i-1].Timestamp < *logEvents[i].Timestamp, "timestamp is not sorted correctly")
	}
}

//
//  Pusher Mocks
//

// Need to remove the tmp state folder after testing.
func newMockPusher() (*pusher, string) {
	logger := zap.NewNop()
	tmpfolder, _ := ioutil.TempDir("", "")
	svc := NewAlwaysPassMockLogClient()
	p := newPusher(&logGroup, &logStreamName, svc, logger)
	return p, tmpfolder
}

//
// Pusher Tests
//

var timestampInMillis = time.Now().UnixNano() / 1e6
var msg = "test log message"

func TestPusher_newLogEventBatch(t *testing.T) {
	p, tmpFolder := newMockPusher()
	defer os.RemoveAll(tmpFolder)

	logEventBatch := p.newLogEventBatch()
	assert.Equal(t, int64(0), logEventBatch.maxTimestampInMillis)
	assert.Equal(t, int64(0), logEventBatch.minTimestampInMillis)
	assert.Equal(t, 0, logEventBatch.byteTotal)
	assert.Equal(t, 0, len(logEventBatch.PutLogEventsInput.LogEvents))
	assert.Equal(t, p.logStreamName, logEventBatch.PutLogEventsInput.LogStreamName)
	assert.Equal(t, p.logGroupName, logEventBatch.PutLogEventsInput.LogGroupName)
	assert.Equal(t, (*string)(nil), logEventBatch.PutLogEventsInput.SequenceToken)
}

func TestPusher_newLogEventBatchIfNeeded(t *testing.T) {
	p, tmpFolder := newMockPusher()
	defer os.RemoveAll(tmpFolder)

	cap := cap(p.logEventBatch.PutLogEventsInput.LogEvents)
	logEvent := NewLogEvent(timestampInMillis, msg)

	for i := 0; i < cap; i++ {
		p.logEventBatch.PutLogEventsInput.LogEvents = append(p.logEventBatch.PutLogEventsInput.LogEvents, logEvent.InputLogEvent)
	}

	assert.Equal(t, cap, len(p.logEventBatch.PutLogEventsInput.LogEvents))

	p.newLogEventBatchIfNeeded(logEvent)
	//the actual log event add operation happens after the func newLogEventBatchIfNeeded
	assert.Equal(t, 0, len(p.logEventBatch.PutLogEventsInput.LogEvents))

	p.logEventBatch.byteTotal = MaxRequestPayloadBytes - logEvent.eventPayloadBytes() + 1
	p.newLogEventBatchIfNeeded(logEvent)
	assert.Equal(t, 0, len(p.logEventBatch.PutLogEventsInput.LogEvents))

	p.logEventBatch.minTimestampInMillis, p.logEventBatch.maxTimestampInMillis = timestampInMillis, timestampInMillis
	p.newLogEventBatchIfNeeded(NewLogEvent(timestampInMillis+(time.Hour*24+time.Millisecond*1).Nanoseconds()/1e6, msg))
	assert.Equal(t, 0, len(p.logEventBatch.PutLogEventsInput.LogEvents))

	//even the event batch is expired, the total byte is sitll 0 at this time.
	p.newLogEventBatchIfNeeded(nil)
	assert.Equal(t, 0, len(p.logEventBatch.PutLogEventsInput.LogEvents))

	//even the event batch is expired, the total byte is sitll 0 at this time.
	p.newLogEventBatchIfNeeded(logEvent)
	assert.Equal(t, 0, len(p.logEventBatch.PutLogEventsInput.LogEvents))

	//the previous sleep is still in effect at this step.
	p.logEventBatch.byteTotal = 1
	p.newLogEventBatchIfNeeded(nil)
	assert.Equal(t, 0, len(p.logEventBatch.PutLogEventsInput.LogEvents))

}

func TestPusher_addLogEvent(t *testing.T) {
	p, tmpFolder := newMockPusher()
	defer os.RemoveAll(tmpFolder)

	p.addLogEvent(NewLogEvent(time.Now().Add(-(14*24+1)*time.Hour).UnixNano()/1e6, msg))
	assert.Equal(t, 0, len(p.logEventBatch.PutLogEventsInput.LogEvents))
	assert.Equal(t, int64(0), p.logEventBatch.minTimestampInMillis)
	assert.Equal(t, int64(0), p.logEventBatch.maxTimestampInMillis)
	assert.Equal(t, 0, p.logEventBatch.byteTotal)

	p.addLogEvent(NewLogEvent(time.Now().Add((2+1)*time.Hour).UnixNano()/1e6, msg))
	assert.Equal(t, 0, len(p.logEventBatch.PutLogEventsInput.LogEvents))
	assert.Equal(t, int64(0), p.logEventBatch.minTimestampInMillis)
	assert.Equal(t, int64(0), p.logEventBatch.maxTimestampInMillis)
	assert.Equal(t, 0, p.logEventBatch.byteTotal)

	p.addLogEvent(NewLogEvent(timestampInMillis, ""))
	assert.Equal(t, 0, len(p.logEventBatch.PutLogEventsInput.LogEvents))
	assert.Equal(t, int64(0), p.logEventBatch.minTimestampInMillis)
	assert.Equal(t, int64(0), p.logEventBatch.maxTimestampInMillis)
	assert.Equal(t, 0, p.logEventBatch.byteTotal)

	p.addLogEvent(NewLogEvent(timestampInMillis, msg))
	assert.Equal(t, 1, len(p.logEventBatch.PutLogEventsInput.LogEvents))
	assert.Equal(t, timestampInMillis, p.logEventBatch.minTimestampInMillis)
	assert.Equal(t, timestampInMillis, p.logEventBatch.maxTimestampInMillis)
	assert.Equal(t, len(msg)+PerEventHeaderBytes, p.logEventBatch.byteTotal)

	p.addLogEvent(NewLogEvent(timestampInMillis+1, msg+"1"))
	assert.Equal(t, 2, len(p.logEventBatch.PutLogEventsInput.LogEvents))
	assert.Equal(t, timestampInMillis, p.logEventBatch.minTimestampInMillis)
	assert.Equal(t, timestampInMillis+1, p.logEventBatch.maxTimestampInMillis)
	assert.Equal(t, len(msg)+len(msg+"1")+2*PerEventHeaderBytes, p.logEventBatch.byteTotal)
}

func TestPusher_truncateLogEvent(t *testing.T) {
	p, tmpFolder := newMockPusher()
	defer os.RemoveAll(tmpFolder)
	largeEventContent := strings.Repeat("a", MaxEventPayloadBytes)

	logEvent := NewLogEvent(timestampInMillis, largeEventContent)
	expectedTruncatedContent := (*logEvent.InputLogEvent.Message)[0:(MaxEventPayloadBytes-PerEventHeaderBytes-len(TruncatedSuffix))] + TruncatedSuffix

	p.AddLogEntry(logEvent)

	assert.Equal(t, expectedTruncatedContent, *logEvent.InputLogEvent.Message)
}
