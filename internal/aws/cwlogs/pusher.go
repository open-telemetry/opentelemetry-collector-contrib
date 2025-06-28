// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package cwlogs // import "github.com/open-telemetry/opentelemetry-collector-contrib/internal/aws/cwlogs"

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
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

	truncatedSuffix = "[Truncated...]"

	eventTimestampLimitInPast  = 14 * 24 * time.Hour // None of the log events in the batch can be older than 14 days
	evenTimestampLimitInFuture = -2 * time.Hour      // None of the log events in the batch can be more than 2 hours in the future.
)

var maxEventPayloadBytes = defaultMaxEventPayloadBytes

// Event struct to present a log event.
type Event struct {
	InputLogEvent types.InputLogEvent
	// The time which log generated.
	GeneratedTime time.Time
	// Identify what is the stream of destination of this event
	StreamKey
}

// NewEvent creates a new log event
// logType will be propagated to LogEventBatch and used by logPusher to determine which client to call PutLogEvent
func NewEvent(timestampMs int64, message string) *Event {
	event := &Event{
		InputLogEvent: types.InputLogEvent{
			Timestamp: aws.Int64(timestampMs),
			Message:   aws.String(message),
		},
	}
	return event
}

// Uniquely identify a cloudwatch logs stream
type StreamKey struct {
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
func newEventBatch(key StreamKey) *eventBatch {
	return &eventBatch{
		putLogEventsInput: &cloudwatchlogs.PutLogEventsInput{
			LogGroupName:  aws.String(key.LogGroupName),
			LogStreamName: aws.String(key.LogStreamName),
			LogEvents:     make([]types.InputLogEvent, 0, maxRequestEventCount),
		},
	}
}

func (batch *eventBatch) exceedsLimit(nextByteTotal int) bool {
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

type ByTimestamp []types.InputLogEvent

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
	AddLogEntry(ctx context.Context, logEvent *Event) error
	ForceFlush(ctx context.Context) error
}

// Struct of logPusher implemented Pusher interface.
type logPusher struct {
	logger *zap.Logger
	// log group name of the current logPusher
	logGroupName *string
	// log stream name of the current logPusher
	logStreamName *string

	logEventBatch *eventBatch

	svcStructuredLog Client
	retryCnt         int
}

// NewPusher creates a logPusher instance
func NewPusher(streamKey StreamKey, retryCnt int,
	svcStructuredLog Client, logger *zap.Logger,
) Pusher {
	pusher := newLogPusher(streamKey, svcStructuredLog, logger)

	pusher.retryCnt = defaultRetryCount
	if retryCnt > 0 {
		pusher.retryCnt = retryCnt
	}

	return pusher
}

// Only create a logPusher, but not start the instance.
func newLogPusher(streamKey StreamKey,
	svcStructuredLog Client, logger *zap.Logger,
) *logPusher {
	pusher := &logPusher{
		logGroupName:     aws.String(streamKey.LogGroupName),
		logStreamName:    aws.String(streamKey.LogStreamName),
		svcStructuredLog: svcStructuredLog,
		logger:           logger,
	}
	pusher.logEventBatch = newEventBatch(streamKey)

	return pusher
}

// AddLogEntry Besides the limit specified by PutLogEvents API, there are some overall limit for the cloudwatchlogs
// listed here: http://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/cloudwatch_limits_cwl.html
//
// Need to pay attention to the below 2 limits:
// Event size 256 KB (maximum). This limit cannot be changed.
// Batch size 1 MB (maximum). This limit cannot be changed.
func (p *logPusher) AddLogEntry(ctx context.Context, logEvent *Event) error {
	var err error
	if logEvent != nil {
		err = logEvent.Validate(p.logger)
		if err != nil {
			return err
		}
		prevBatch := p.addLogEvent(logEvent)
		if prevBatch != nil {
			err = p.pushEventBatch(ctx, prevBatch)
		}
	}
	return err
}

func (p *logPusher) ForceFlush(ctx context.Context) error {
	prevBatch := p.renewEventBatch()
	if prevBatch != nil {
		return p.pushEventBatch(ctx, prevBatch)
	}
	return nil
}

func (p *logPusher) pushEventBatch(ctx context.Context, req any) error {
	// http://docs.aws.amazon.com/goto/SdkForGoV1/logs-2014-03-28/PutLogEvents
	// The log events in the batch must be in chronological ordered by their
	// timestamp (the time the event occurred, expressed as the number of milliseconds
	// since Jan 1, 1970 00:00:00 UTC).
	logEventBatch := req.(*eventBatch)
	logEventBatch.sortLogEvents()
	putLogEventsInput := logEventBatch.putLogEventsInput

	startTime := time.Now()

	err := p.svcStructuredLog.PutLogEvents(ctx, putLogEventsInput, p.retryCnt)
	if err != nil {
		return err
	}

	p.logger.Debug("logpusher: publish log events successfully.",
		zap.Int("NumOfLogEvents", len(putLogEventsInput.LogEvents)),
		zap.Float64("LogEventsSize", float64(logEventBatch.byteTotal)/float64(1024)),
		zap.Int64("Time", time.Since(startTime).Nanoseconds()/int64(time.Millisecond)))

	return nil
}

func (p *logPusher) addLogEvent(logEvent *Event) *eventBatch {
	if logEvent == nil {
		return nil
	}

	var prevBatch *eventBatch
	currentBatch := p.logEventBatch
	if currentBatch.exceedsLimit(logEvent.eventPayloadBytes()) || !currentBatch.isActive(logEvent.InputLogEvent.Timestamp) {
		prevBatch = currentBatch
		currentBatch = newEventBatch(StreamKey{
			LogGroupName:  *p.logGroupName,
			LogStreamName: *p.logStreamName,
		})
	}
	currentBatch.append(logEvent)
	p.logEventBatch = currentBatch

	return prevBatch
}

func (p *logPusher) renewEventBatch() *eventBatch {
	var prevBatch *eventBatch
	if len(p.logEventBatch.putLogEventsInput.LogEvents) > 0 {
		prevBatch = p.logEventBatch
		p.logEventBatch = newEventBatch(StreamKey{
			LogGroupName:  *p.logGroupName,
			LogStreamName: *p.logStreamName,
		})
	}

	return prevBatch
}

// A Pusher that is able to send events to multiple streams.
type multiStreamPusher struct {
	logStreamManager LogStreamManager
	client           Client
	pusherMap        map[StreamKey]Pusher
	logger           *zap.Logger
}

func newMultiStreamPusher(logStreamManager LogStreamManager, client Client, logger *zap.Logger) *multiStreamPusher {
	return &multiStreamPusher{
		logStreamManager: logStreamManager,
		client:           client,
		logger:           logger,
		pusherMap:        make(map[StreamKey]Pusher),
	}
}

func (m *multiStreamPusher) AddLogEntry(ctx context.Context, event *Event) error {
	if err := m.logStreamManager.InitStream(ctx, event.StreamKey); err != nil {
		return err
	}

	var pusher Pusher
	var ok bool

	if pusher, ok = m.pusherMap[event.StreamKey]; !ok {
		pusher = NewPusher(event.StreamKey, 1, m.client, m.logger)
		m.pusherMap[event.StreamKey] = pusher
	}

	return pusher.AddLogEntry(ctx, event)
}

func (m *multiStreamPusher) ForceFlush(ctx context.Context) error {
	var errs []error

	for _, val := range m.pusherMap {
		err := val.ForceFlush(ctx)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return errors.Join(errs...)
	}

	return nil
}

// Factory for a Pusher that has capability of sending events to multiple log streams
type MultiStreamPusherFactory interface {
	CreateMultiStreamPusher() Pusher
}

type multiStreamPusherFactory struct {
	logStreamManager LogStreamManager
	logger           *zap.Logger
	client           Client
}

// Creates a new MultiStreamPusherFactory
func NewMultiStreamPusherFactory(logStreamManager LogStreamManager, client Client, logger *zap.Logger) MultiStreamPusherFactory {
	return &multiStreamPusherFactory{
		logStreamManager: logStreamManager,
		client:           client,
		logger:           logger,
	}
}

// Factory method to create a Pusher that has support to sending events to multiple log streams
func (msf *multiStreamPusherFactory) CreateMultiStreamPusher() Pusher {
	return newMultiStreamPusher(msf.logStreamManager, msf.client, msf.logger)
}

// Manages the creation of streams
type LogStreamManager interface {
	// Initialize a stream so that it can receive logs
	// This will make sure that the stream exists and if it does not exist,
	// It will create one. Implementations of this method MUST be safe for concurrent use.
	InitStream(ctx context.Context, streamKey StreamKey) error
}

type logStreamManager struct {
	logStreamMutex sync.Mutex
	streams        map[StreamKey]bool
	client         Client
}

func NewLogStreamManager(svcStructuredLog Client) LogStreamManager {
	return &logStreamManager{
		client:  svcStructuredLog,
		streams: make(map[StreamKey]bool),
	}
}

func (lsm *logStreamManager) InitStream(ctx context.Context, streamKey StreamKey) error {
	if _, ok := lsm.streams[streamKey]; !ok {
		lsm.logStreamMutex.Lock()
		defer lsm.logStreamMutex.Unlock()

		if _, ok := lsm.streams[streamKey]; !ok {
			err := lsm.client.CreateStream(ctx, &streamKey.LogGroupName, &streamKey.LogStreamName)
			lsm.streams[streamKey] = true
			return err
		}
	}
	return nil
	// does not do anything if stream already exists
}
