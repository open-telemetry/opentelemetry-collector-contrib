package awsemfexporter

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter/publisher"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
)

const (
	//http://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/cloudwatch_limits_cwl.html
	//In truncation logic, it assuming this constant value is larger than PerEventHeaderBytes + len(TruncatedSuffix)
	MaxEventPayloadBytes = 1024 * 256 //256KB
	// http://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_PutLogEvents.html
	MaxRequestEventCount   = 10000
	PerEventHeaderBytes    = 26
	MaxRequestPayloadBytes = 1024 * 1024 * 1

	logEventChanBufferSize    = 10000 // 1 request can handle max 10000 log entries
	minPusherIntervalInMillis = 200   // 5 TPS

	logEventBatchPushChanBufferSize = 2 // processing part does not need to be blocked by the current put log event request
	TruncatedSuffix                 = "[Truncated...]"

	Structured    = "Structured"
)

//Struct to present a log event.
type LogEvent struct {
	InputLogEvent *cloudwatchlogs.InputLogEvent
	//The file name where this log event comes from
	FileName string
	//The offset for the input file
	FilePosition int64
	//The time which log generated.
	LogGeneratedTime time.Time
	//Indicate this log event is a starter of new multiline.
	multiLineStart bool
	//Indicate the log type, valid values are "Structured" or empty ""
	logType string
}

//Calculate the log event payload bytes.
func (logEvent *LogEvent) eventPayloadBytes() int {
	return len(*logEvent.InputLogEvent.Message) + PerEventHeaderBytes
}

func (logEvent *LogEvent) truncateIfNeeded() bool {
	if logEvent.eventPayloadBytes() > MaxEventPayloadBytes {
		log.Printf("W! logpusher: the single log event size is %v, which is larger than the max event payload allowed %v. Truncate the log event.", logEvent.eventPayloadBytes(), MaxEventPayloadBytes)
		newPayload := (*logEvent.InputLogEvent.Message)[0:(MaxEventPayloadBytes - PerEventHeaderBytes - len(TruncatedSuffix))]
		newPayload += TruncatedSuffix
		logEvent.InputLogEvent.Message = &newPayload
		return true
	}
	return false
}

//Create a new log event
//logType will be propagated to logEventBatch and used by pusher to determine which client to call PutLogEvent
func NewLogEvent(timestampInMillis int64, message string, filename string, position int64, logType string) *LogEvent {
	logEvent := &LogEvent{
		InputLogEvent: &cloudwatchlogs.InputLogEvent{
			Timestamp: aws.Int64(timestampInMillis),
			Message:   aws.String(message)},
		FileName:     filename,
		FilePosition: position,
		logType:      logType,
	}
	return logEvent
}

//Struct to present a log event batch
type LogEventBatch struct {
	PutLogEventsInput *cloudwatchlogs.PutLogEventsInput
	//the lastest file name for this log event.
	FileName string
	//the latest offset for this log file.
	FilePosition int64
	//the total bytes already in this log event batch
	byteTotal int
	//min timestamp recorded in this log event batch (ms)
	minTimestampInMillis int64
	//max timestamp recorded in this log event batch (ms)
	maxTimestampInMillis int64

	creationTime time.Time
	// Indicate the logType of a LogEventBatch, valid values are "Structured" or empty ""
	logType string
}

/**
 * A batch of log events in a single request cannot span more than 24 hours.
 * Otherwise, the operation fails.
 */
func (logEventBatch *LogEventBatch) timestampWithin24Hours(targetInMillis *int64) bool {
	//new log event batch
	if logEventBatch.minTimestampInMillis == 0 || logEventBatch.maxTimestampInMillis == 0 {
		return true
	}
	if *targetInMillis-logEventBatch.minTimestampInMillis > 24*3600*1e3 {
		return false
	}
	if logEventBatch.maxTimestampInMillis-*targetInMillis > 24*3600*1e3 {
		return false
	}
	return true
}

//Sort the log events based on the timestamp.
func (logEventBatch *LogEventBatch) sortLogEvents() {
	inputLogEvents := logEventBatch.PutLogEventsInput.LogEvents
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

//Pusher is one per log group
type Pusher interface {
	AddLogEntry(logEvent *LogEvent)
}

//Struct of pusher implemented Pusher interface.
type pusher struct {
	//log group name for the current pusher
	logGroupName *string
	//log stream name for the current pusher
	logStreamName *string
	//file folder where to store the offset of files.
	fileStateFolder *string

	forceFlushInterval time.Duration

	svcStructuredLog LogClient
	streamToken      string //no init value

	logEventChan chan *LogEvent
	pushChan     chan *LogEventBatch
	shutdownChan <-chan bool

	pushTicker     *time.Ticker
	tmpEventTicker *time.Ticker
	logEventBatch  *LogEventBatch
	publisher      *publisher.Publisher
	wg             *sync.WaitGroup
	retryCnt       int
}

//Create a pusher instance and start the instance afterwards
func NewPusher(logGroupName, logStreamName, fileStateFolder *string,
	forceFlushInterval time.Duration, retryCnt int,
	svcStructuredLog LogClient, shutdownChan <-chan bool, wg *sync.WaitGroup) Pusher {

	pusher := newPusher(logGroupName, logStreamName, fileStateFolder,
		forceFlushInterval,
		svcStructuredLog, shutdownChan, wg)

	// For blocking queue, assuming the log batch payload size is 1MB. Set queue size to 2
	// For nonblocking queue, assuming the log batch payload size is much less than 1MB. Set queue size to 20
	var queue publisher.Queue
	queue = publisher.NewNonBlockingFifoQueue(20)
	pusher.retryCnt = defaultRetryCount
	if retryCnt > 0 {
		pusher.retryCnt = retryCnt
	}

	pusher.publisher, _ = publisher.NewPublisher(queue, 1, 2*time.Second, pusher.pushLogEventBatch)

	pusher.startRoutines()

	return pusher
}

//Only create a pusher, but not start the instance.
func newPusher(logGroupName, logStreamName, fileStateFolder *string,
	forceFlushInterval time.Duration,
	svcStructuredLog LogClient, shutdownChan <-chan bool, wg *sync.WaitGroup) *pusher {
	pusher := &pusher{
		logGroupName:       logGroupName,
		logStreamName:      logStreamName,
		fileStateFolder:    fileStateFolder,
		forceFlushInterval: forceFlushInterval,
		svcStructuredLog:   svcStructuredLog,
		logEventChan:       make(chan *LogEvent, logEventChanBufferSize),
		pushChan:           make(chan *LogEventBatch, logEventBatchPushChanBufferSize),
		shutdownChan:       shutdownChan,
		wg:                 wg,
		pushTicker:         time.NewTicker(forceFlushInterval),
	}

	pusher.logEventBatch = pusher.newLogEventBatch()
	return pusher
}

func (p *pusher) startRoutines() {
	p.wg.Add(1)
	go p.push()
	go p.processLogEntry()
}

// Besides the limit specified by PutLogEvents API, there are some overall limit for the cloudwatchlogs
// listed here: http://docs.aws.amazon.com/AmazonCloudWatch/latest/logs/cloudwatch_limits_cwl.html
//
// Need to pay attention to the below 2 limits:
// Event size 256 KB (maximum). This limit cannot be changed.
// Batch size 1 MB (maximum). This limit cannot be changed.
func (p *pusher) AddLogEntry(logEvent *LogEvent) {
	if logEvent != nil {
		p.logEventChan <- logEvent
	}
}

//This method processes the log entries if any available in the channel.
//After processing, it will push to the log event batch ready to publish.
func (p *pusher) processLogEntry() {
	var lastAccurateTimestamp int64
	var curLineTruncated bool
	for {
		select {
		case logEvent := <-p.logEventChan:
			if !logEvent.multiLineStart && curLineTruncated {
				log.Println("W! Current logEvent has already been truncated, discard appended event...")
				continue
			}
			curLineTruncated = logEvent.truncateIfNeeded()
			if *logEvent.InputLogEvent.Timestamp == int64(0) {
				if lastAccurateTimestamp != 0 {
					logEvent.InputLogEvent.Timestamp = aws.Int64(lastAccurateTimestamp)
				} else {
					logEvent.InputLogEvent.Timestamp = aws.Int64(logEvent.LogGeneratedTime.UnixNano() / 1e6)
				}
			} else {
				lastAccurateTimestamp = *logEvent.InputLogEvent.Timestamp
			}
			p.addLogEvent(logEvent)
		case <-p.pushTicker.C:
			p.newLogEventBatchIfNeeded(nil)
		case <-p.shutdownChan:
			log.Printf("D! logpusher: process log entry routine receives the shutdown signal, exiting.")
			p.publisher.Close()
			p.wg.Done()
			return
		}
	}
}

//Push the log event batch which is ready to publish.
func (p *pusher) push() {
	for {
		select {
		case logEventBatch := <-p.pushChan:
			p.publisher.Publish(logEventBatch)
		case <-p.shutdownChan:
			log.Printf("D! logpusher: push routine receives the shutdown signal, exiting.")
			return
		}
	}
}

func (p *pusher) pushLogEventBatch(req interface{}) {
	//http://docs.aws.amazon.com/goto/SdkForGoV1/logs-2014-03-28/PutLogEvents
	//* The log events in the batch must be in chronological ordered by their
	//timestamp (the time the event occurred, expressed as the number of milliseconds
	//since Jan 1, 1970 00:00:00 UTC).
	logEventBatch := req.(*LogEventBatch)
	logEventBatch.sortLogEvents()
	putLogEventsInput := logEventBatch.PutLogEventsInput

	if p.streamToken == "" {
		//log part and retry logic are already done inside the CreateStream
		// when the error is not nil, the stream token is "", which is handled in the below logic.
		p.streamToken, _ = p.svcStructuredLog.CreateStream(p.logGroupName, p.logStreamName)
	}

	if p.streamToken != "" {
		putLogEventsInput.SequenceToken = aws.String(p.streamToken)
	}

	startTime := time.Now()

	var tmpToken *string
	tmpToken = p.svcStructuredLog.PutLogEvents(putLogEventsInput, p.retryCnt)


	log.Printf("D! logpusher: publish %d log events with size %f KB in %d ms.",
		len(putLogEventsInput.LogEvents),
		float64(logEventBatch.byteTotal)/float64(1024),
		time.Now().Sub(startTime).Nanoseconds()/1e6)

	if tmpToken != nil {
		p.streamToken = *tmpToken
	}
	diff := time.Now().Sub(startTime)
	if timeLeft := minPusherIntervalInMillis*time.Millisecond - diff; timeLeft > 0 {
		time.Sleep(timeLeft)
	}
}

//Create a new log event batch if needed.
func (p *pusher) newLogEventBatch() *LogEventBatch {
	logEventBatch := &LogEventBatch{
		PutLogEventsInput: &cloudwatchlogs.PutLogEventsInput{
			LogGroupName:  p.logGroupName,
			LogStreamName: p.logStreamName,
			LogEvents:     make([]*cloudwatchlogs.InputLogEvent, 0, MaxRequestEventCount)},
		creationTime: time.Now(),
	}
	return logEventBatch
}

//Determine if a new log event batch is needed.
func (p *pusher) newLogEventBatchIfNeeded(logEvent *LogEvent) {
	logEventBatch := p.logEventBatch
	if eventBatchExpired := time.Now().Sub(logEventBatch.creationTime) > p.forceFlushInterval; eventBatchExpired && logEventBatch.byteTotal > 0 ||
		len(logEventBatch.PutLogEventsInput.LogEvents) == cap(logEventBatch.PutLogEventsInput.LogEvents) ||
		logEvent != nil && (logEventBatch.byteTotal+logEvent.eventPayloadBytes() > MaxRequestPayloadBytes || !logEventBatch.timestampWithin24Hours(logEvent.InputLogEvent.Timestamp)) {
		p.pushChan <- logEventBatch
		p.logEventBatch = p.newLogEventBatch()
	}
}

//Add the log event onto the log event batch
func (p *pusher) addLogEvent(logEvent *LogEvent) {
	if len(*logEvent.InputLogEvent.Message) == 0 {
		return
	}

	//http://docs.aws.amazon.com/goto/SdkForGoV1/logs-2014-03-28/PutLogEvents
	//* None of the log events in the batch can be more than 2 hours in the
	//future.
	//* None of the log events in the batch can be older than 14 days or the
	//retention period of the log group.
	currentTime := time.Now().UTC()
	utcTime := time.Unix(0, *logEvent.InputLogEvent.Timestamp*1e6).UTC()
	duration := currentTime.Sub(utcTime).Hours()
	if duration > 24*14 || duration < -2 {
		log.Printf("E! logpusher: the log entry in (%v/%v) with timestamp (%v) comparing to the current time (%v) is older than 14 days or more than 2 hours in the future. Discard the log entry.", p.logGroupName, logEvent.FileName, utcTime, currentTime)
		return
	}

	p.newLogEventBatchIfNeeded(logEvent)
	logEventBatch := p.logEventBatch

	logEventBatch.PutLogEventsInput.LogEvents = append(logEventBatch.PutLogEventsInput.LogEvents, logEvent.InputLogEvent)
	logEventBatch.byteTotal += logEvent.eventPayloadBytes()
	logEventBatch.FileName = logEvent.FileName
	logEventBatch.FilePosition = logEvent.FilePosition
	if logEventBatch.minTimestampInMillis == 0 || logEventBatch.minTimestampInMillis > *logEvent.InputLogEvent.Timestamp {
		logEventBatch.minTimestampInMillis = *logEvent.InputLogEvent.Timestamp
	}
	if logEventBatch.maxTimestampInMillis == 0 || logEventBatch.maxTimestampInMillis < *logEvent.InputLogEvent.Timestamp {
		logEventBatch.maxTimestampInMillis = *logEvent.InputLogEvent.Timestamp
	}
}
