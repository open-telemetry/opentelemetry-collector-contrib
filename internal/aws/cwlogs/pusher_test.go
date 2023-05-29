// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cwlogs

import (
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"
)

func TestConcurrentPushAndFlush(t *testing.T) {
	maxEventPayloadBytes = 128

	concurrency := 10
	current := time.Now().UnixNano() / 1e6
	collection := map[string]interface{}{}

	emfPusher := newMockPusherWithEventCheck(func(msg string) {
		if _, ok := collection[msg]; ok {
			t.Errorf("Sending duplicated event message %s", msg)
		} else {
			collection[msg] = struct{}{}
		}
	})

	wg := sync.WaitGroup{}
	wg.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func(ii int) {
			for j := 0; j < 10; j++ {
				err := emfPusher.AddLogEntry(NewEvent(current, fmt.Sprintf("batch-%d-%d", ii, j)))
				if err != nil {
					t.Errorf("Error adding log entry: %v", err)
				}
			}
			time.Sleep(1000 * time.Millisecond)
			err := emfPusher.ForceFlush()
			if err != nil {
				t.Errorf("Error flushing: %v", err)

			}
			wg.Done()
		}(i)
	}
	wg.Wait()
	assert.Equal(t, concurrency*10, len(collection))

	maxEventPayloadBytes = defaultMaxEventPayloadBytes
}

func newMockPusherWithEventCheck(check func(msg string)) Pusher {
	svc := newAlwaysPassMockLogClient(func(args mock.Arguments) {
		input := args.Get(0).(*cloudwatchlogs.PutLogEventsInput)
		for _, event := range input.LogEvents {
			eventMsg := *event.Message
			check(eventMsg)
		}
	})
	p := newLogPusher(PusherKey{
		LogGroupName:  logGroup,
		LogStreamName: logStreamName,
	}, *svc, zap.NewNop())
	return p
}

// logEvent Tests
func TestLogEvent_eventPayloadBytes(t *testing.T) {
	testMessage := "test message"
	logEvent := NewEvent(0, testMessage)
	assert.Equal(t, len(testMessage)+perEventHeaderBytes, logEvent.eventPayloadBytes())
}

func TestValidateLogEventWithMutating(t *testing.T) {
	maxEventPayloadBytes = 64

	logEvent := NewEvent(0, "abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789abcdefghijklmnopqrstuvwxyz0123456789")
	logEvent.GeneratedTime = time.Now()
	err := logEvent.Validate(zap.NewNop())
	assert.Nil(t, err)
	assert.True(t, *logEvent.InputLogEvent.Timestamp > int64(0))
	assert.Equal(t, 64-perEventHeaderBytes, len(*logEvent.InputLogEvent.Message))

	maxEventPayloadBytes = defaultMaxEventPayloadBytes
}

func TestValidateLogEventFailed(t *testing.T) {
	logger := zap.NewNop()
	logEvent := NewEvent(0, "")
	err := logEvent.Validate(logger)
	assert.NotNil(t, err)
	assert.Equal(t, "empty log event message", err.Error())

	invalidTimestamp := time.Now().AddDate(0, -1, 0)
	logEvent = NewEvent(invalidTimestamp.Unix()*1e3, "test")
	err = logEvent.Validate(logger)
	assert.NotNil(t, err)
	assert.Equal(t, "the log entry's timestamp is older than 14 days or more than 2 hours in the future", err.Error())
}

// eventBatch Tests
func TestLogEventBatch_timestampWithin24Hours(t *testing.T) {
	min := time.Date(2017, time.June, 20, 23, 38, 0, 0, time.Local)
	max := min.Add(23 * time.Hour)
	logEventBatch := &eventBatch{
		maxTimestampMs: max.UnixNano() / 1e6,
		minTimestampMs: min.UnixNano() / 1e6,
	}

	// less than the min
	target := min.Add(-1 * time.Hour)
	assert.True(t, logEventBatch.isActive(aws.Int64(target.UnixNano()/1e6)))

	target = target.Add(-1 * time.Millisecond)
	assert.False(t, logEventBatch.isActive(aws.Int64(target.UnixNano()/1e6)))

	// more than the max
	target = max.Add(1 * time.Hour)
	assert.True(t, logEventBatch.isActive(aws.Int64(target.UnixNano()/1e6)))

	target = target.Add(1 * time.Millisecond)
	assert.False(t, logEventBatch.isActive(aws.Int64(target.UnixNano()/1e6)))

	// in between min and max
	target = min.Add(2 * time.Hour)
	assert.True(t, logEventBatch.isActive(aws.Int64(target.UnixNano()/1e6)))
}

func TestLogEventBatch_sortLogEvents(t *testing.T) {
	totalEvents := 10
	logEventBatch := &eventBatch{
		putLogEventsInput: &cloudwatchlogs.PutLogEventsInput{
			LogEvents: make([]*cloudwatchlogs.InputLogEvent, 0, totalEvents)}}

	for i := 0; i < totalEvents; i++ {
		timestamp := rand.Int()
		logEvent := NewEvent(
			int64(timestamp),
			fmt.Sprintf("message%v", timestamp))
		fmt.Printf("logEvents[%d].Timestamp=%d.\n", i, timestamp)
		logEventBatch.putLogEventsInput.LogEvents = append(logEventBatch.putLogEventsInput.LogEvents, logEvent.InputLogEvent)
	}

	logEventBatch.sortLogEvents()

	logEvents := logEventBatch.putLogEventsInput.LogEvents
	for i := 1; i < totalEvents; i++ {
		fmt.Printf("logEvents[%d].Timestamp=%d, logEvents[%d].Timestamp=%d.\n", i-1, *logEvents[i-1].Timestamp, i, *logEvents[i].Timestamp)
		assert.True(t, *logEvents[i-1].Timestamp < *logEvents[i].Timestamp, "timestamp is not sorted correctly")
	}
}

//
//  pusher Mocks
//

// Need to remove the tmp state folder after testing.
func newMockPusher() *logPusher {
	svc := newAlwaysPassMockLogClient(func(args mock.Arguments) {})
	return newLogPusher(PusherKey{
		LogGroupName:  logGroup,
		LogStreamName: logStreamName,
	}, *svc, zap.NewNop())
}

//
// pusher Tests
//

var timestampMs = time.Now().UnixNano() / 1e6
var msg = "test log message"

func TestPusher_newLogEventBatch(t *testing.T) {
	p := newMockPusher()

	logEventBatch := newEventBatch(PusherKey{
		LogGroupName:  logGroup,
		LogStreamName: logStreamName,
	})
	assert.Equal(t, int64(0), logEventBatch.maxTimestampMs)
	assert.Equal(t, int64(0), logEventBatch.minTimestampMs)
	assert.Equal(t, 0, logEventBatch.byteTotal)
	assert.Equal(t, 0, len(logEventBatch.putLogEventsInput.LogEvents))
	assert.Equal(t, p.logStreamName, logEventBatch.putLogEventsInput.LogStreamName)
	assert.Equal(t, p.logGroupName, logEventBatch.putLogEventsInput.LogGroupName)
	assert.Equal(t, (*string)(nil), logEventBatch.putLogEventsInput.SequenceToken)
}

func TestPusher_addLogEventBatch(t *testing.T) {
	p := newMockPusher()

	cap := cap(p.logEventBatch.putLogEventsInput.LogEvents)
	logEvent := NewEvent(timestampMs, msg)

	for i := 0; i < cap; i++ {
		p.logEventBatch.putLogEventsInput.LogEvents = append(p.logEventBatch.putLogEventsInput.LogEvents, logEvent.InputLogEvent)
	}

	assert.Equal(t, cap, len(p.logEventBatch.putLogEventsInput.LogEvents))

	assert.NotNil(t, p.addLogEvent(logEvent))
	// the actual log event add operation happens after the func newLogEventBatchIfNeeded
	assert.Equal(t, 1, len(p.logEventBatch.putLogEventsInput.LogEvents))

	p.logEventBatch.byteTotal = maxRequestPayloadBytes - logEvent.eventPayloadBytes() + 1
	assert.NotNil(t, p.addLogEvent(logEvent))
	assert.Equal(t, 1, len(p.logEventBatch.putLogEventsInput.LogEvents))

	p.logEventBatch.minTimestampMs, p.logEventBatch.maxTimestampMs = timestampMs, timestampMs
	assert.NotNil(t, p.addLogEvent(NewEvent(timestampMs+(time.Hour*24+time.Millisecond*1).Nanoseconds()/1e6, msg)))
	assert.Equal(t, 1, len(p.logEventBatch.putLogEventsInput.LogEvents))

	assert.Nil(t, p.addLogEvent(nil))
	assert.Equal(t, 1, len(p.logEventBatch.putLogEventsInput.LogEvents))

	assert.NotNil(t, p.addLogEvent(logEvent))
	assert.Equal(t, 1, len(p.logEventBatch.putLogEventsInput.LogEvents))

	p.logEventBatch.byteTotal = 1
	assert.Nil(t, p.addLogEvent(nil))
	assert.Equal(t, 1, len(p.logEventBatch.putLogEventsInput.LogEvents))

}

func TestAddLogEventWithValidation(t *testing.T) {
	p := newMockPusher()
	largeEventContent := strings.Repeat("a", defaultMaxEventPayloadBytes)

	logEvent := NewEvent(timestampMs, largeEventContent)
	expectedTruncatedContent := (*logEvent.InputLogEvent.Message)[0:(defaultMaxEventPayloadBytes-perEventHeaderBytes-len(truncatedSuffix))] + truncatedSuffix

	err := p.AddLogEntry(logEvent)
	if err != nil {
		t.Errorf("Error adding log entry: %v", err)
	}
	assert.Equal(t, expectedTruncatedContent, *logEvent.InputLogEvent.Message)

	logEvent = NewEvent(timestampMs, "")
	assert.NotNil(t, p.addLogEvent(logEvent))
}
