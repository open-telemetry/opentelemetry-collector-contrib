package awsemfexporter

import (
	"fmt"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/awsemfexporter/publisher"
	"io/ioutil"
	"math/rand"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/stretchr/testify/assert"
)

//
//  LogEvent Tests
//
func TestLogEvent_eventPayloadBytes(t *testing.T) {
	testMessage := "test message"
	logEvent := NewLogEvent(0, testMessage, "", 0, "")
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
			fmt.Sprintf("message%v", timestamp),
			"FileName",
			int64(timestamp),
			"")
		fmt.Printf("logEvents[%d].Timetsmap=%d.\n", i, timestamp)
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
func NewMockPusher() (*pusher, string, chan<- bool) {
	tmpfolder, _ := ioutil.TempDir("", "")
	shutdownChan := make(chan bool)
	svc := NewAlwaysPassMockLogClient()
	wg := sync.WaitGroup{}
	wg.Add(1)
	p := newPusher(&logGroup, &logStreamName, &tmpfolder, time.Second, svc, shutdownChan, &wg)
	p.publisher, _ = publisher.NewPublisher(publisher.NewNonBlockingFifoQueue(1), 1, 2*time.Second, p.pushLogEventBatch)
	return p, tmpfolder, shutdownChan
}

//
// Pusher Tests
//

var timestampInMillis = time.Now().UnixNano() / 1e6
var msg = "test log message"
var fileName = "/tmp/logfile.log"
var fileOffset = int64(123)

func TestPusher_processLogEntry(t *testing.T) {
	p, tmpFolder, shutdownChan := NewMockPusher()
	defer os.RemoveAll(tmpFolder)

	logEvent := NewLogEvent(timestampInMillis, msg, fileName, fileOffset, "")
	//fake the log event batch is almost full
	p.logEventBatch.byteTotal = MaxRequestPayloadBytes - 2*logEvent.eventPayloadBytes() + 1
	go p.processLogEntry()

	p.logEventChan <- logEvent
	logEvent = NewLogEvent(timestampInMillis, msg, fileName, fileOffset, "")
	p.logEventChan <- logEvent

	time.Sleep(time.Millisecond * 100)
	close(shutdownChan)

	select {
	case <-p.logEventChan:
		assert.Fail(t, "The log event should be already consumed.")
	default:
	}

	select {
	case <-p.pushChan:
	default:
		assert.Fail(t, "The log event batch chan (push chan) should have entry.")
	}

}

func TestPusher_processLogEntryforZeroTimestamp(t *testing.T) {
	p, tmpFolder, shutdownChan := NewMockPusher()
	defer os.RemoveAll(tmpFolder)

	logEvent := NewLogEvent(0, msg, fileName, fileOffset, "")
	//fake the log event batch is almost full
	logEvent.LogGeneratedTime = time.Unix(int64(timestampInMillis/1e3), 0)
	p.logEventBatch.byteTotal = MaxRequestPayloadBytes - 2*logEvent.eventPayloadBytes() + 1
	go p.processLogEntry()

	p.logEventChan <- logEvent
	logEvent = NewLogEvent(0, msg, fileName, fileOffset, "")
	logEvent.LogGeneratedTime = time.Unix(int64(timestampInMillis/1e3), 0)
	p.logEventChan <- logEvent

	time.Sleep(time.Millisecond * 100)
	close(shutdownChan)
	assert.Equal(t, int64(timestampInMillis/1e3)*1e3, *logEvent.InputLogEvent.Timestamp)
	select {
	case <-p.logEventChan:
		assert.Fail(t, "The log event should be already consumed.")
	default:
	}

	select {
	case <-p.pushChan:
	default:
		assert.Fail(t, "The log event batch chan (push chan) should have entry.")
	}
}

func TestPusher_push(t *testing.T) {
	p, tmpFolder, shutdownChan := NewMockPusher()
	defer os.RemoveAll(tmpFolder)

	go p.push()

	p.pushChan <- p.newLogEventBatch()
	time.Sleep(time.Millisecond * 100)
	close(shutdownChan)

	select {
	case <-p.pushChan:
		assert.Fail(t, "The log event batch in the push chan should be already consumed by push() func.")
	default:
	}
}

func TestPusher_newLogEventBatch(t *testing.T) {
	p, tmpFolder, _ := NewMockPusher()
	defer os.RemoveAll(tmpFolder)

	logEventBatch := p.newLogEventBatch()
	assert.Equal(t, int64(0), logEventBatch.maxTimestampInMillis)
	assert.Equal(t, int64(0), logEventBatch.minTimestampInMillis)
	assert.Equal(t, 0, logEventBatch.byteTotal)
	assert.Equal(t, 0, len(logEventBatch.PutLogEventsInput.LogEvents))
	assert.Equal(t, p.logStreamName, logEventBatch.PutLogEventsInput.LogStreamName)
	assert.Equal(t, p.logGroupName, logEventBatch.PutLogEventsInput.LogGroupName)
	assert.Equal(t, (*string)(nil), logEventBatch.PutLogEventsInput.SequenceToken)
	assert.Equal(t, int64(0), logEventBatch.FilePosition)
	assert.Equal(t, "", logEventBatch.FileName)
}

func TestPusher_newLogEventBatchIfNeeded(t *testing.T) {
	p, tmpFolder, _ := NewMockPusher()
	defer os.RemoveAll(tmpFolder)

	cap := cap(p.logEventBatch.PutLogEventsInput.LogEvents)
	logEvent := NewLogEvent(timestampInMillis, msg, fileName, fileOffset, "")

	for i := 0; i < cap; i++ {
		p.logEventBatch.PutLogEventsInput.LogEvents = append(p.logEventBatch.PutLogEventsInput.LogEvents, logEvent.InputLogEvent)
	}

	assert.Equal(t, cap, len(p.logEventBatch.PutLogEventsInput.LogEvents))

	p.newLogEventBatchIfNeeded(logEvent)
	//the actual log event add operation happens after the func newLogEventBatchIfNeeded
	assert.Equal(t, 0, len(p.logEventBatch.PutLogEventsInput.LogEvents))
	select {
	case <-p.pushChan:
	default:
		assert.Fail(t, "A log event batch should be retrieved.")
	}

	p.logEventBatch.byteTotal = MaxRequestPayloadBytes - logEvent.eventPayloadBytes() + 1
	p.newLogEventBatchIfNeeded(logEvent)
	assert.Equal(t, 0, len(p.logEventBatch.PutLogEventsInput.LogEvents))
	select {
	case <-p.pushChan:
	default:
		assert.Fail(t, "A log event batch should be retrieved.")
	}

	p.logEventBatch.minTimestampInMillis, p.logEventBatch.maxTimestampInMillis = timestampInMillis, timestampInMillis
	p.newLogEventBatchIfNeeded(NewLogEvent(timestampInMillis+(time.Hour*24+time.Millisecond*1).Nanoseconds()/1e6, msg, fileName, fileOffset, ""))
	assert.Equal(t, 0, len(p.logEventBatch.PutLogEventsInput.LogEvents))
	select {
	case <-p.pushChan:
	default:
		assert.Fail(t, "A log event batch should be retrieved.")
	}

	time.Sleep(p.forceFlushInterval + time.Second)
	//even the event batch is expired, the total byte is sitll 0 at this time.
	p.newLogEventBatchIfNeeded(nil)
	assert.Equal(t, 0, len(p.logEventBatch.PutLogEventsInput.LogEvents))
	select {
	case <-p.pushChan:
		assert.Fail(t, "A log event batch should not be retrieved.")
	default:
	}

	//even the event batch is expired, the total byte is sitll 0 at this time.
	p.newLogEventBatchIfNeeded(logEvent)
	assert.Equal(t, 0, len(p.logEventBatch.PutLogEventsInput.LogEvents))
	select {
	case <-p.pushChan:
		assert.Fail(t, "A log event batch should not be retrieved.")
	default:
	}

	//the previous sleep is still in effect at this step.
	p.logEventBatch.byteTotal = 1
	p.newLogEventBatchIfNeeded(nil)
	assert.Equal(t, 0, len(p.logEventBatch.PutLogEventsInput.LogEvents))
	select {
	case <-p.pushChan:
	default:
		assert.Fail(t, "A log event batch should be retrieved.")
	}

}

func TestPusher_addLogEvent(t *testing.T) {
	p, tmpFolder, _ := NewMockPusher()
	defer os.RemoveAll(tmpFolder)

	p.addLogEvent(NewLogEvent(time.Now().Add(-(14*24+1)*time.Hour).UnixNano()/1e6, msg, fileName, fileOffset, ""))
	assert.Equal(t, 0, len(p.logEventBatch.PutLogEventsInput.LogEvents))
	assert.Equal(t, int64(0), p.logEventBatch.minTimestampInMillis)
	assert.Equal(t, int64(0), p.logEventBatch.maxTimestampInMillis)
	assert.Equal(t, "", p.logEventBatch.FileName)
	assert.Equal(t, int64(0), p.logEventBatch.FilePosition)
	assert.Equal(t, 0, p.logEventBatch.byteTotal)

	p.addLogEvent(NewLogEvent(time.Now().Add((2+1)*time.Hour).UnixNano()/1e6, msg, fileName, fileOffset, ""))
	assert.Equal(t, 0, len(p.logEventBatch.PutLogEventsInput.LogEvents))
	assert.Equal(t, int64(0), p.logEventBatch.minTimestampInMillis)
	assert.Equal(t, int64(0), p.logEventBatch.maxTimestampInMillis)
	assert.Equal(t, "", p.logEventBatch.FileName)
	assert.Equal(t, int64(0), p.logEventBatch.FilePosition)
	assert.Equal(t, 0, p.logEventBatch.byteTotal)

	p.addLogEvent(NewLogEvent(timestampInMillis, "", fileName, fileOffset, ""))
	assert.Equal(t, 0, len(p.logEventBatch.PutLogEventsInput.LogEvents))
	assert.Equal(t, int64(0), p.logEventBatch.minTimestampInMillis)
	assert.Equal(t, int64(0), p.logEventBatch.maxTimestampInMillis)
	assert.Equal(t, "", p.logEventBatch.FileName)
	assert.Equal(t, int64(0), p.logEventBatch.FilePosition)
	assert.Equal(t, 0, p.logEventBatch.byteTotal)

	p.addLogEvent(NewLogEvent(timestampInMillis, msg, fileName, fileOffset, ""))
	assert.Equal(t, 1, len(p.logEventBatch.PutLogEventsInput.LogEvents))
	assert.Equal(t, timestampInMillis, p.logEventBatch.minTimestampInMillis)
	assert.Equal(t, timestampInMillis, p.logEventBatch.maxTimestampInMillis)
	assert.Equal(t, fileName, p.logEventBatch.FileName)
	assert.Equal(t, fileOffset, p.logEventBatch.FilePosition)
	assert.Equal(t, len(msg)+PerEventHeaderBytes, p.logEventBatch.byteTotal)

	p.addLogEvent(NewLogEvent(timestampInMillis+1, msg+"1", fileName+"1", fileOffset+1, ""))
	assert.Equal(t, 2, len(p.logEventBatch.PutLogEventsInput.LogEvents))
	assert.Equal(t, timestampInMillis, p.logEventBatch.minTimestampInMillis)
	assert.Equal(t, timestampInMillis+1, p.logEventBatch.maxTimestampInMillis)
	assert.Equal(t, fileName+"1", p.logEventBatch.FileName)
	assert.Equal(t, fileOffset+1, p.logEventBatch.FilePosition)
	assert.Equal(t, len(msg)+len(msg+"1")+2*PerEventHeaderBytes, p.logEventBatch.byteTotal)
}

func TestPusher_truncateLogEvent(t *testing.T) {
	p, tmpFolder, shutdownChan := NewMockPusher()
	defer os.RemoveAll(tmpFolder)
	largeEventContent := strings.Repeat("a", MaxEventPayloadBytes)

	logEvent := NewLogEvent(timestampInMillis, largeEventContent, fileName, MaxEventPayloadBytes, "")
	logEvent.multiLineStart = true
	expectedTruncatedContent := (*logEvent.InputLogEvent.Message)[0:(MaxEventPayloadBytes-PerEventHeaderBytes-len(TruncatedSuffix))] + TruncatedSuffix

	go p.processLogEntry()

	p.logEventChan <- logEvent

	time.Sleep(time.Millisecond * 100)
	close(shutdownChan)
	assert.Equal(t, expectedTruncatedContent, *logEvent.InputLogEvent.Message)
	select {
	case <-p.logEventChan:
		assert.Fail(t, "The log event should be already consumed.")
	default:
	}
}

