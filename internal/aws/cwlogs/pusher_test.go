// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cwlogs

import (
	"context"
	"fmt"
	"math/rand/v2"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

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
	assert.NoError(t, err)
	assert.Positive(t, *logEvent.InputLogEvent.Timestamp)
	assert.Len(t, *logEvent.InputLogEvent.Message, 64-perEventHeaderBytes)

	maxEventPayloadBytes = defaultMaxEventPayloadBytes
}

func TestValidateLogEventFailed(t *testing.T) {
	logger := zap.NewNop()
	logEvent := NewEvent(0, "")
	err := logEvent.Validate(logger)
	assert.Error(t, err)
	assert.Equal(t, "empty log event message", err.Error())

	invalidTimestamp := time.Now().AddDate(0, -1, 0)
	logEvent = NewEvent(invalidTimestamp.Unix()*1e3, "test")
	err = logEvent.Validate(logger)
	assert.Error(t, err)
	assert.Equal(t, "the log entry's timestamp is older than 14 days or more than 2 hours in the future", err.Error())
}

// eventBatch Tests
func TestLogEventBatch_timestampWithin24Hours(t *testing.T) {
	minDate := time.Date(2017, time.June, 20, 23, 38, 0, 0, time.Local)
	maxDate := minDate.Add(23 * time.Hour)
	logEventBatch := &eventBatch{
		maxTimestampMs: maxDate.UnixNano() / 1e6,
		minTimestampMs: minDate.UnixNano() / 1e6,
	}

	// less than the min
	target := minDate.Add(-1 * time.Hour)
	assert.True(t, logEventBatch.isActive(aws.Int64(target.UnixNano()/1e6)))

	target = target.Add(-1 * time.Millisecond)
	assert.False(t, logEventBatch.isActive(aws.Int64(target.UnixNano()/1e6)))

	// more than the max
	target = maxDate.Add(1 * time.Hour)
	assert.True(t, logEventBatch.isActive(aws.Int64(target.UnixNano()/1e6)))

	target = target.Add(1 * time.Millisecond)
	assert.False(t, logEventBatch.isActive(aws.Int64(target.UnixNano()/1e6)))

	// in between min and max
	target = minDate.Add(2 * time.Hour)
	assert.True(t, logEventBatch.isActive(aws.Int64(target.UnixNano()/1e6)))
}

func TestLogEventBatch_sortLogEvents(t *testing.T) {
	totalEvents := 10
	logEventBatch := &eventBatch{
		putLogEventsInput: &cloudwatchlogs.PutLogEventsInput{
			LogEvents: make([]types.InputLogEvent, 0, totalEvents),
		},
	}

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
		assert.Less(t, *logEvents[i-1].Timestamp, *logEvents[i].Timestamp, "timestamp is not sorted correctly")
	}
}

//
//  pusher Mocks
//

// Need to remove the tmp state folder after testing.
func newMockPusher() *logPusher {
	return newLogPusher(StreamKey{
		LogGroupName:  logGroup,
		LogStreamName: logStreamName,
	}, Client{svc: &mockCloudWatchClient{}}, zap.NewNop())
}

//
// pusher Tests
//

var (
	timestampMs = time.Now().UnixNano() / 1e6
	msg         = "test log message"
)

func TestPusher_newLogEventBatch(t *testing.T) {
	p := newMockPusher()

	logEventBatch := newEventBatch(StreamKey{
		LogGroupName:  logGroup,
		LogStreamName: logStreamName,
	})
	assert.Equal(t, int64(0), logEventBatch.maxTimestampMs)
	assert.Equal(t, int64(0), logEventBatch.minTimestampMs)
	assert.Equal(t, 0, logEventBatch.byteTotal)
	assert.Empty(t, logEventBatch.putLogEventsInput.LogEvents)
	assert.Equal(t, p.logStreamName, logEventBatch.putLogEventsInput.LogStreamName)
	assert.Equal(t, p.logGroupName, logEventBatch.putLogEventsInput.LogGroupName)
	assert.Equal(t, (*string)(nil), logEventBatch.putLogEventsInput.SequenceToken)
}

func TestPusher_addLogEventBatch(t *testing.T) {
	p := newMockPusher()

	c := cap(p.logEventBatch.putLogEventsInput.LogEvents)
	logEvent := NewEvent(timestampMs, msg)

	for i := 0; i < c; i++ {
		p.logEventBatch.putLogEventsInput.LogEvents = append(p.logEventBatch.putLogEventsInput.LogEvents, logEvent.InputLogEvent)
	}

	assert.Len(t, p.logEventBatch.putLogEventsInput.LogEvents, c)

	assert.NotNil(t, p.addLogEvent(logEvent))
	// the actual log event add operation happens after the func newLogEventBatchIfNeeded
	assert.Len(t, p.logEventBatch.putLogEventsInput.LogEvents, 1)

	p.logEventBatch.byteTotal = maxRequestPayloadBytes - logEvent.eventPayloadBytes() + 1
	assert.NotNil(t, p.addLogEvent(logEvent))
	assert.Len(t, p.logEventBatch.putLogEventsInput.LogEvents, 1)

	p.logEventBatch.minTimestampMs, p.logEventBatch.maxTimestampMs = timestampMs, timestampMs
	assert.NotNil(t, p.addLogEvent(NewEvent(timestampMs+(time.Hour*24+time.Millisecond*1).Nanoseconds()/1e6, msg)))
	assert.Len(t, p.logEventBatch.putLogEventsInput.LogEvents, 1)

	assert.Nil(t, p.addLogEvent(nil))
	assert.Len(t, p.logEventBatch.putLogEventsInput.LogEvents, 1)

	assert.NotNil(t, p.addLogEvent(logEvent))
	assert.Len(t, p.logEventBatch.putLogEventsInput.LogEvents, 1)

	p.logEventBatch.byteTotal = 1
	assert.Nil(t, p.addLogEvent(nil))
	assert.Len(t, p.logEventBatch.putLogEventsInput.LogEvents, 1)
}

func TestAddLogEventWithValidation(t *testing.T) {
	p := newMockPusher()
	largeEventContent := strings.Repeat("a", defaultMaxEventPayloadBytes)

	logEvent := NewEvent(timestampMs, largeEventContent)
	expectedTruncatedContent := (*logEvent.InputLogEvent.Message)[0:(defaultMaxEventPayloadBytes-perEventHeaderBytes-len(truncatedSuffix))] + truncatedSuffix

	require.NoError(t, p.AddLogEntry(context.Background(), logEvent), "Error adding log entry")
	assert.Equal(t, expectedTruncatedContent, *logEvent.InputLogEvent.Message)

	logEvent = NewEvent(timestampMs, "")
	assert.NotNil(t, p.addLogEvent(logEvent))
}

func TestStreamManager(t *testing.T) {
	mockCwAPI := &mockCloudWatchClient{
		createLogStream: func(_ context.Context, _ *cloudwatchlogs.CreateLogStreamInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error) {
			return &cloudwatchlogs.CreateLogStreamOutput{}, nil
		},
	}
	manager := NewLogStreamManager(Client{
		svc: mockCwAPI,
	})

	// Verify that the stream is created in the first time
	assert.NoError(t, manager.InitStream(context.Background(), StreamKey{
		LogGroupName:  "foo",
		LogStreamName: "bar",
	}))

	assert.Equal(t, int32(1), mockCwAPI.createLogStreamCount.Load())

	// Verify that the stream is not created in the second time
	assert.NoError(t, manager.InitStream(context.Background(), StreamKey{
		LogGroupName:  "foo",
		LogStreamName: "bar",
	}))

	assert.Equal(t, int32(1), mockCwAPI.createLogStreamCount.Load())

	// Verify that a different stream is created
	assert.NoError(t, manager.InitStream(context.Background(), StreamKey{
		LogGroupName:  "foo",
		LogStreamName: "bar2",
	}))

	assert.Equal(t, int32(2), mockCwAPI.createLogStreamCount.Load())
}

func TestMultiStreamFactory(t *testing.T) {
	mockCwAPI := &mockCloudWatchClient{
		createLogStream: func(_ context.Context, _ *cloudwatchlogs.CreateLogStreamInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error) {
			return &cloudwatchlogs.CreateLogStreamOutput{}, nil
		},
	}
	manager := NewLogStreamManager(Client{svc: mockCwAPI})
	factory := NewMultiStreamPusherFactory(manager, Client{svc: mockCwAPI}, nil)
	pusher := factory.CreateMultiStreamPusher()

	assert.IsType(t, &multiStreamPusher{}, pusher)
}

func TestMultiStreamPusher(t *testing.T) {
	mockCwAPI := &mockCloudWatchClient{
		createLogStream: func(_ context.Context, _ *cloudwatchlogs.CreateLogStreamInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.CreateLogStreamOutput, error) {
			return &cloudwatchlogs.CreateLogStreamOutput{}, nil
		},
		putLogEvents: func(_ context.Context, _ *cloudwatchlogs.PutLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.PutLogEventsOutput, error) {
			return &cloudwatchlogs.PutLogEventsOutput{}, nil
		},
	}

	manager := NewLogStreamManager(Client{svc: mockCwAPI})
	zap := zap.NewNop()
	pusher := newMultiStreamPusher(manager, Client{svc: mockCwAPI}, zap)
	event := NewEvent(time.Now().UnixMilli(), "testing")
	event.LogGroupName = "foo"
	event.LogStreamName = "bar"
	event.GeneratedTime = time.Now()

	assert.NoError(t, pusher.AddLogEntry(context.Background(), event))
	assert.NoError(t, pusher.AddLogEntry(context.Background(), event))
	assert.Equal(t, int32(0), mockCwAPI.putLogEventsCount.Load())
	assert.NoError(t, pusher.ForceFlush(context.Background()))

	assert.Equal(t, int32(1), mockCwAPI.createLogStreamCount.Load())
	assert.Equal(t, int32(2), mockCwAPI.putLogEventsCount.Load())

	event2 := NewEvent(time.Now().UnixMilli(), "testing")
	event2.LogGroupName = "foo"
	event2.LogStreamName = "bar2"
	event2.GeneratedTime = time.Now()

	assert.NoError(t, pusher.AddLogEntry(context.Background(), event2))
	assert.NoError(t, pusher.ForceFlush(context.Background()))

	assert.Equal(t, int32(2), mockCwAPI.createLogStreamCount.Load())
	assert.Equal(t, int32(4), mockCwAPI.putLogEventsCount.Load())
}
