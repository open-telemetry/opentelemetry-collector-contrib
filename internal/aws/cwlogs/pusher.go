// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cwlogs // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/cwlogs"

import (
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"go.uber.org/zap"
)

const (
	// http://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/cloudwatch_limits_cwl.html
	// In truncation logic, it assuming this constant value is larger than perEventHeaderBytes + len(truncatedSuffix)
	defaultMaxEventPayloadBytes = 1024 * 256 // 256KB
	// http://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_PutLogEvents.html
	maxRequestEventCount   = 10000
	perEventHeaderBytes    = 26
	maxRequestPayloadBytes = 1024 * 1024 * 1

	minPusherIntervalMs = 200 // 5 TPS

	truncatedSuffix = "[Truncated...]"

	eventTimestampLimitInPast  = 14 * 24 * time.Hour // None of the log events in the batch can be older than 14 days
	evenTimestampLimitInFuture = -2 * time.Hour      // None of the log events in the batch can be more than 2 hours in the future.
)

var (
	maxEventPayloadBytes = defaultMaxEventPayloadBytes
)

// Event struct to present a log event.
type Event struct {
	InputLogEvent *cloudwatchlogs.InputLogEvent
	// The time which log generated.
	GeneratedTime time.Time
	LogGroupName  string
	LogStreamName string
}

// NewEvent creates a new log event
// logType will be propagated to LogEventBatch and used by logPusher to determine which client to call PutLogEvent
func NewEvent(timestampMs int64, message string) *Event {
	event := &Event{
		InputLogEvent: &cloudwatchlogs.InputLogEvent{
			Timestamp: aws.Int64(timestampMs),
			Message:   aws.String(message)},
	}
	return event
}

type PusherKey struct {
	LogGroupName  string
	LogStreamName string
}

func (logEvent *Event) Validate(logger *zap.Logger) error {
	if logEvent.eventPayloadBytes() > maxEventPayloadBytes {
		logger.Warn("logpusher: the single log event size is larger than the max event payload allowed. Truncate the log event.",
			zap.Int("SingleLogEventSize", logEvent.eventPayloadBytes()), zap.Int("maxEventPayloadBytes", maxEventPayloadBytes))

		newPayload := (*logEvent.InputLogEvent.Message)[0:(maxEventPayloadBytes - perEventHeaderBytes - len(truncatedSuffix))]
		newPayload += truncatedSuffix
		logEvent.InputLogEvent.Message = &newPayload
	}

	if *logEvent.InputLogEvent.Timestamp == int64(0) {
		logEvent.InputLogEvent.Timestamp = aws.Int64(logEvent.GeneratedTime.UnixNano() / int64(time.Millisecond))
	}
	if len(*logEvent.InputLogEvent.Message) == 0 {
		return errors.New("empty log event message")
	}

	// http://docs.aws.amazon.com/goto/SdkForGoV1/logs-2014-03-28/PutLogEvents
	// * None of the log events in the batch can be more than 2 hours in the
	// future.
	// * None of the log events in the batch can be older than 14 days or the
	// retention period of the log group.
	currentTime := time.Now().UTC()
	utcTime := time.Unix(0, *logEvent.InputLogEvent.Timestamp*int64(time.Millisecond)).UTC()
	duration := currentTime.Sub(utcTime)
	if duration > eventTimestampLimitInPast || duration < evenTimestampLimitInFuture {
		err := errors.New("the log entry's timestamp is older than 14 days or more than 2 hours in the future")
		logger.Error("discard log entry with invalid timestamp",
			zap.Error(err), zap.String("LogEventTimestamp", utcTime.String()), zap.String("CurrentTime", currentTime.String()))
		return err
	}
	return nil
}

// Calculate the log event payload bytes.
func (logEvent *Event) eventPayloadBytes() int {
	return len(*logEvent.InputLogEvent.Message) + perEventHeaderBytes
}

// eventBatch struct to present a log event batch
type eventBatch struct {
	putLogEventsInput *cloudwatchlogs.PutLogEventsInput
	// the total bytes already in this log event batch
	byteTotal int
	// min timestamp recorded in this log event batch (ms)
	minTimestampMs int64
	// max timestamp recorded in this log event batch (ms)
	maxTimestampMs int64
}

// Create a new log event batch if needed.
func newEventBatch(key PusherKey) *eventBatch {
	return &eventBatch{
		putLogEventsInput: &cloudwatchlogs.PutLogEventsInput{
			LogGroupName:  aws.String(key.LogGroupName),
			LogStreamName: aws.String(key.LogStreamName),
			LogEvents:     make([]*cloudwatchlogs.InputLogEvent, 0, maxRequestEventCount)},
	}
}

func (batch eventBatch) exceedsLimit(nextByteTotal int) bool {
	return len(batch.putLogEventsInput.LogEvents) == cap(batch.putLogEventsInput.LogEvents) ||
		batch.byteTotal+nextByteTotal > maxEventPayloadBytes
}

// isActive checks whether the eventBatch spans more than 24 hours. Returns
// false if the condition does not match, and this batch should not be processed
// any further.
func (batch *eventBatch) isActive(targetTimestampMs *int64) bool {
	// new log event batch
	if batch.minTimestampMs == 0 || batch.maxTimestampMs == 0 {
		return true
	}
	if *targetTimestampMs-batch.minTimestampMs > 24*3600*1e3 {
		return false
	}
	if batch.maxTimestampMs-*targetTimestampMs > 24*3600*1e3 {
		return false
	}
	return true
}

func (batch *eventBatch) append(event *Event) {
	batch.putLogEventsInput.LogEvents = append(batch.putLogEventsInput.LogEvents, event.InputLogEvent)
	batch.byteTotal += event.eventPayloadBytes()
	if batch.minTimestampMs == 0 || batch.minTimestampMs > *event.InputLogEvent.Timestamp {
		batch.minTimestampMs = *event.InputLogEvent.Timestamp
	}
	if batch.maxTimestampMs == 0 || batch.maxTimestampMs < *event.InputLogEvent.Timestamp {
		batch.maxTimestampMs = *event.InputLogEvent.Timestamp
	}
}

// Sort the log events based on the timestamp.
func (batch *eventBatch) sortLogEvents() {
	inputLogEvents := batch.putLogEventsInput.LogEvents
	sort.Stable(ByTimestamp(inputLogEvents))
}

type ByTimestamp []*cloudwatchlogs.InputLogEvent

func (inputLogEvents ByTimestamp) Len() int {
	return len(inputLogEvents)
}

func (inputLogEvents ByTimestamp) Swap(i, j int) {
	inputLogEvents[i], inputLogEvents[j] = inputLogEvents[j], inputLogEvents[i]
}

func (inputLogEvents ByTimestamp) Less(i, j int) bool {
	return *inputLogEvents[i].Timestamp < *inputLogEvents[j].Timestamp
}

// Pusher is created by log group and log stream
type Pusher interface {
	AddLogEntry(logEvent *Event) error
	ForceFlush() error
}

// Struct of logPusher implemented Pusher interface.
type logPusher struct {
	logger *zap.Logger
	// log group name of the current logPusher
	logGroupName *string
	// log stream name of the current logPusher
	logStreamName *string

	batchUpdateLock sync.Mutex
	logEventBatch   *eventBatch

	pushLock         sync.Mutex
	streamToken      string // no init value
	svcStructuredLog Client
	retryCnt         int
}

// NewPusher creates a logPusher instance
func NewPusher(pusherKey PusherKey, retryCnt int,
	svcStructuredLog Client, logger *zap.Logger) Pusher {

	pusher := newLogPusher(pusherKey, svcStructuredLog, logger)

	pusher.retryCnt = defaultRetryCount
	if retryCnt > 0 {
		pusher.retryCnt = retryCnt
	}

	return pusher
}

// Only create a logPusher, but not start the instance.
func newLogPusher(pusherKey PusherKey,
	svcStructuredLog Client, logger *zap.Logger) *logPusher {
	pusher := &logPusher{
		logGroupName:     aws.String(pusherKey.LogGroupName),
		logStreamName:    aws.String(pusherKey.LogStreamName),
		svcStructuredLog: svcStructuredLog,
		logger:           logger,
	}
	pusher.logEventBatch = newEventBatch(pusherKey)

	return pusher
}

// AddLogEntry Besides the limit specified by PutLogEvents API, there are some overall limit for the cloudwatchlogs
// listed here: http://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/cloudwatch_limits_cwl.html
//
// Need to pay attention to the below 2 limits:
// Event size 256 KB (maximum). This limit cannot be changed.
// Batch size 1 MB (maximum). This limit cannot be changed.
func (p *logPusher) AddLogEntry(logEvent *Event) error {
	var err error
	if logEvent != nil {
		err = logEvent.Validate(p.logger)
		if err != nil {
			return err
		}
		prevBatch := p.addLogEvent(logEvent)
		if prevBatch != nil {
			err = p.pushEventBatch(prevBatch)
		}
	}
	return err
}

func (p *logPusher) ForceFlush() error {
	prevBatch := p.renewEventBatch()
	if prevBatch != nil {
		return p.pushEventBatch(prevBatch)
	}
	return nil
}

func (p *logPusher) pushEventBatch(req interface{}) error {
	p.pushLock.Lock()
	defer p.pushLock.Unlock()

	// http://docs.aws.amazon.com/goto/SdkForGoV1/logs-2014-03-28/PutLogEvents
	// The log events in the batch must be in chronological ordered by their
	// timestamp (the time the event occurred, expressed as the number of milliseconds
	// since Jan 1, 1970 00:00:00 UTC).
	logEventBatch := req.(*eventBatch)
	logEventBatch.sortLogEvents()
	putLogEventsInput := logEventBatch.putLogEventsInput

	if p.streamToken == "" {
		var err error
		// log part and retry logic are already done inside the CreateStream
		// when the error is not nil, the stream token is "", which is handled in the below logic.
		p.streamToken, err = p.svcStructuredLog.CreateStream(p.logGroupName, p.logStreamName)
		// TODO Known issue: createStream will fail if the corresponding logGroup and logStream has been created.
		// The retry mechanism helps get the first stream token, yet the first batch will be sent twice in this situation.
		if err != nil {
			p.logger.Warn("Failed to create stream token", zap.Error(err))
		}
	}

	if p.streamToken != "" {
		putLogEventsInput.SequenceToken = aws.String(p.streamToken)
	}

	startTime := time.Now()

	var tmpToken *string
	var err error
	tmpToken, err = p.svcStructuredLog.PutLogEvents(putLogEventsInput, p.retryCnt)

	if err != nil {
		return err
	}

	p.logger.Info("logpusher: publish log events successfully.",
		zap.Int("NumOfLogEvents", len(putLogEventsInput.LogEvents)),
		zap.Float64("LogEventsSize", float64(logEventBatch.byteTotal)/float64(1024)),
		zap.Int64("Time", time.Since(startTime).Nanoseconds()/int64(time.Millisecond)))

	if tmpToken != nil {
		p.streamToken = *tmpToken
	}
	diff := time.Since(startTime)
	if timeLeft := minPusherIntervalMs*time.Millisecond - diff; timeLeft > 0 {
		time.Sleep(timeLeft)
	}
	return nil
}

func (p *logPusher) addLogEvent(logEvent *Event) *eventBatch {
	if logEvent == nil {
		return nil
	}

	p.batchUpdateLock.Lock()
	defer p.batchUpdateLock.Unlock()

	var prevBatch *eventBatch
	currentBatch := p.logEventBatch
	if currentBatch.exceedsLimit(logEvent.eventPayloadBytes()) || !currentBatch.isActive(logEvent.InputLogEvent.Timestamp) {
		prevBatch = currentBatch
		currentBatch = newEventBatch(PusherKey{
			LogGroupName:  *p.logGroupName,
			LogStreamName: *p.logStreamName,
		})
	}
	currentBatch.append(logEvent)
	p.logEventBatch = currentBatch

	return prevBatch
}

func (p *logPusher) renewEventBatch() *eventBatch {
	p.batchUpdateLock.Lock()
	defer p.batchUpdateLock.Unlock()

	var prevBatch *eventBatch
	if len(p.logEventBatch.putLogEventsInput.LogEvents) > 0 {
		prevBatch = p.logEventBatch
		p.logEventBatch = newEventBatch(PusherKey{
			LogGroupName:  *p.logGroupName,
			LogStreamName: *p.logStreamName,
		})
	}

	return prevBatch
}
