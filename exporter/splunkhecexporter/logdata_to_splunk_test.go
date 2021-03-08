// Copyright 2020, OpenTelemetry Authors
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

package splunkhecexporter

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

func Test_chunkEvents(t *testing.T) {
	logger := zap.NewNop()
	ts := pdata.TimestampUnixNano(123)

	tests := []struct {
		name             string
		logDataFn        func() pdata.Logs
		configDataFn     func() *Config
		wantSplunkEvents []*splunk.Event
	}{
		{
			name: "valid",
			logDataFn: func() pdata.Logs {
				logRecord := pdata.NewLogRecord()
				logRecord.Body().SetStringVal("mylog")
				logRecord.Attributes().InsertString(conventions.AttributeServiceName, "myapp")
				logRecord.Attributes().InsertString(splunk.SourcetypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
				logRecord.Attributes().InsertString("custom", "custom")
				logRecord.SetTimestamp(ts)
				return makeLog(logRecord)
			},
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent("mylog", ts, map[string]interface{}{"custom": "custom"}, "myhost", "myapp", "myapp-type"),
			},
		},
		{
			name: "non-string attribute",
			logDataFn: func() pdata.Logs {
				logRecord := pdata.NewLogRecord()
				logRecord.Body().SetStringVal("mylog")
				logRecord.Attributes().InsertString(conventions.AttributeServiceName, "myapp")
				logRecord.Attributes().InsertString(splunk.SourcetypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
				logRecord.Attributes().InsertDouble("foo", 123)
				logRecord.SetTimestamp(ts)
				return makeLog(logRecord)
			},
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent("mylog", ts, map[string]interface{}{"foo": float64(123)}, "myhost", "myapp", "myapp-type"),
			},
		},
		{
			name: "with_config",
			logDataFn: func() pdata.Logs {
				logRecord := pdata.NewLogRecord()
				logRecord.Body().SetStringVal("mylog")
				logRecord.Attributes().InsertString("custom", "custom")
				logRecord.SetTimestamp(ts)
				return makeLog(logRecord)
			},
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent("mylog", ts, map[string]interface{}{"custom": "custom"}, "unknown", "source", "sourcetype"),
			},
		},
		{
			name: "log_is_empty",
			logDataFn: func() pdata.Logs {
				logRecord := pdata.NewLogRecord()
				return makeLog(logRecord)
			},
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent(nil, 0, nil, "unknown", "source", "sourcetype"),
			},
		},
		{
			name: "with double body",
			logDataFn: func() pdata.Logs {
				logRecord := pdata.NewLogRecord()
				logRecord.Body().SetDoubleVal(42)
				logRecord.Attributes().InsertString(conventions.AttributeServiceName, "myapp")
				logRecord.Attributes().InsertString(splunk.SourcetypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
				logRecord.Attributes().InsertString("custom", "custom")
				logRecord.SetTimestamp(ts)
				return makeLog(logRecord)
			},
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent(float64(42), ts, map[string]interface{}{"custom": "custom"}, "myhost", "myapp", "myapp-type"),
			},
		},
		{
			name: "with int body",
			logDataFn: func() pdata.Logs {
				logRecord := pdata.NewLogRecord()
				logRecord.Body().SetIntVal(42)
				logRecord.Attributes().InsertString(conventions.AttributeServiceName, "myapp")
				logRecord.Attributes().InsertString(splunk.SourcetypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
				logRecord.Attributes().InsertString("custom", "custom")
				logRecord.SetTimestamp(ts)
				return makeLog(logRecord)
			},
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent(int64(42), ts, map[string]interface{}{"custom": "custom"}, "myhost", "myapp", "myapp-type"),
			},
		},
		{
			name: "with bool body",
			logDataFn: func() pdata.Logs {
				logRecord := pdata.NewLogRecord()
				logRecord.Body().SetBoolVal(true)
				logRecord.Attributes().InsertString(conventions.AttributeServiceName, "myapp")
				logRecord.Attributes().InsertString(splunk.SourcetypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
				logRecord.Attributes().InsertString("custom", "custom")
				logRecord.SetTimestamp(ts)
				return makeLog(logRecord)
			},
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent(true, ts, map[string]interface{}{"custom": "custom"}, "myhost", "myapp", "myapp-type"),
			},
		},
		{
			name: "with map body",
			logDataFn: func() pdata.Logs {
				logRecord := pdata.NewLogRecord()
				attVal := pdata.NewAttributeValueMap()
				attMap := attVal.MapVal()
				attMap.InsertDouble("23", 45)
				attMap.InsertString("foo", "bar")
				attVal.CopyTo(logRecord.Body())
				logRecord.Attributes().InsertString(conventions.AttributeServiceName, "myapp")
				logRecord.Attributes().InsertString(splunk.SourcetypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
				logRecord.Attributes().InsertString("custom", "custom")
				logRecord.SetTimestamp(ts)
				return makeLog(logRecord)
			},
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent(map[string]interface{}{"23": float64(45), "foo": "bar"}, ts, map[string]interface{}{"custom": "custom"}, "myhost", "myapp", "myapp-type"),
			},
		},
		{
			name: "with nil body",
			logDataFn: func() pdata.Logs {
				logRecord := pdata.NewLogRecord()
				logRecord.Attributes().InsertString(conventions.AttributeServiceName, "myapp")
				logRecord.Attributes().InsertString(splunk.SourcetypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
				logRecord.Attributes().InsertString("custom", "custom")
				logRecord.SetTimestamp(ts)
				return makeLog(logRecord)
			},
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent(nil, ts, map[string]interface{}{"custom": "custom"}, "myhost", "myapp", "myapp-type"),
			},
		},
		{
			name: "with array body",
			logDataFn: func() pdata.Logs {
				logRecord := pdata.NewLogRecord()
				attVal := pdata.NewAttributeValueArray()
				attArray := attVal.ArrayVal()
				attArray.Append(pdata.NewAttributeValueString("foo"))
				attVal.CopyTo(logRecord.Body())
				logRecord.Attributes().InsertString(conventions.AttributeServiceName, "myapp")
				logRecord.Attributes().InsertString(splunk.SourcetypeLabel, "myapp-type")
				logRecord.Attributes().InsertString(conventions.AttributeHostName, "myhost")
				logRecord.Attributes().InsertString("custom", "custom")
				logRecord.SetTimestamp(ts)
				return makeLog(logRecord)
			},
			configDataFn: func() *Config {
				return &Config{
					Source:     "source",
					SourceType: "sourcetype",
				}
			},
			wantSplunkEvents: []*splunk.Event{
				commonLogSplunkEvent([]interface{}{"foo"}, ts, map[string]interface{}{"custom": "custom"}, "myhost", "myapp", "myapp-type"),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs := logDataWrapper{&[]pdata.Logs{tt.logDataFn()}[0]}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			chunkCh := logs.chunkEvents(ctx, logger, tt.configDataFn())

			events := bytes.Split(bytes.TrimSpace((<-chunkCh).buf.Bytes()), []byte("\r\n\r\n"))

			require.Equal(t, len(tt.wantSplunkEvents), len(events))

			var got splunk.Event
			var gots []*splunk.Event

			for i, event := range events {
				json.Unmarshal(event, &got)
				want := tt.wantSplunkEvents[i]
				// float64 back to int64. int64 unmarshalled to float64 because Event is interface{}.
				if _, ok := want.Event.(int64); ok {
					if g, ok := got.Event.(float64); ok {
						got.Event = int64(g)
					}
				}
				assert.EqualValues(t, tt.wantSplunkEvents[i], &got)
				gots = append(gots, &got)
			}
			assert.Equal(t, tt.wantSplunkEvents, gots)
		})
	}
}

func Test_chunkEvents_MaxContentLength_AllEventsInChunk(t *testing.T) {
	logs := testLogs()

	_, max, events := jsonEncodeEventsBytes(logs, &Config{})

	eventsLength := 0
	for _, event := range events {
		eventsLength += len(event)
	}

	// Chunk max content length to fit all events in 1 chunk.
	chunkLength := len(events) * max
	config := Config{MaxContentLength: chunkLength}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	chunkCh := logs.chunkEvents(ctx, zap.NewNop(), &config)

	numChunks := 0

	for chunk := range chunkCh {
		assert.Nil(t, chunk.err)
		assert.Len(t, chunk.buf.Bytes(), eventsLength)
		numChunks++
	}

	assert.Equal(t, 1, numChunks)
}

func Test_chunkEvents_MaxContentLength_0(t *testing.T) {
	logs := testLogs()

	_, _, events := jsonEncodeEventsBytes(logs, &Config{})

	eventsLength := 0
	for _, event := range events {
		eventsLength += len(event)
	}

	// Chunk max content length 0 is interpreted as unlimited length.
	chunkLength := 0
	config := Config{MaxContentLength: chunkLength}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	chunkCh := logs.chunkEvents(ctx, zap.NewNop(), &config)

	numChunks := 0

	for chunk := range chunkCh {
		assert.Nil(t, chunk.err)
		assert.Len(t, chunk.buf.Bytes(), eventsLength)
		numChunks++
	}

	assert.Equal(t, 1, numChunks)
}

func Test_chunkEvents_MaxContentLength_0_Cancel(t *testing.T) {
	logs := testLogs()

	_, _, events := jsonEncodeEventsBytes(logs, &Config{})

	eventsLength := 0
	for _, event := range events {
		eventsLength += len(event)
	}

	// Chunk max content length 0 is interpreted as unlimited length.
	chunkLength := 0
	config := Config{MaxContentLength: chunkLength}

	ctx, cancel := context.WithCancel(context.Background())

	chunkCh := logs.chunkEvents(ctx, zap.NewNop(), &config)

	cancel()

	// Giving time for chunkCh to close.
	time.Sleep(time.Millisecond)

	numChunks := 0

	for chunk := range chunkCh {
		assert.Nil(t, chunk.buf)
		assert.Nil(t, chunk.err)
		numChunks++
	}

	assert.Equal(t, 0, numChunks)
}

func Test_chunkEvents_MaxContentLength_Negative(t *testing.T) {
	logs := testLogs()

	_, _, events := jsonEncodeEventsBytes(logs, &Config{})

	eventsLength := 0
	for _, event := range events {
		eventsLength += len(event)
	}

	// Negative max content length is interpreted as unlimited length.
	chunkLength := -3
	config := Config{MaxContentLength: chunkLength}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	chunkCh := logs.chunkEvents(ctx, zap.NewNop(), &config)

	numChunks := 0

	for chunk := range chunkCh {
		assert.Nil(t, chunk.err)
		assert.Len(t, chunk.buf.Bytes(), eventsLength)
		numChunks++
	}

	assert.Equal(t, 1, numChunks)
}

func Test_chunkEvents_MaxContentLength_Small_Error(t *testing.T) {
	logs := testLogs()

	min, _, _ := jsonEncodeEventsBytes(logs, &Config{})

	// Configuring max content length to be smaller than event lengths.
	chunkLength := min - 1
	config := Config{MaxContentLength: chunkLength}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	chunkCh := logs.chunkEvents(ctx, zap.NewNop(), &config)

	numChunks := 0

	for chunk := range chunkCh {
		if chunk.err == nil {
			numChunks++
		}
		assert.Nil(t, chunk.buf)
		assert.Contains(t, chunk.err.Error(), "log event bytes exceed max content length configured")
	}

	assert.Equal(t, 0, numChunks)
}

func Test_chunkEvents_MaxContentLength_Small_Error_Cancel(t *testing.T) {
	logs := testLogs()

	min, _, _ := jsonEncodeEventsBytes(logs, &Config{})

	// Configuring max content length to be smaller than event lengths.
	chunkLength := min - 1
	config := Config{MaxContentLength: chunkLength}

	ctx, cancel := context.WithCancel(context.Background())

	chunkCh := logs.chunkEvents(ctx, zap.NewNop(), &config)

	cancel()

	// Giving time for chunkCh to close.
	time.Sleep(time.Millisecond)

	numChunks := 0

	for chunk := range chunkCh {
		if chunk.err == nil {
			numChunks++
		}
		assert.Nil(t, chunk.buf)
		assert.Nil(t, chunk.err)
	}

	assert.Equal(t, 0, numChunks)
}

func Test_chunkEvents_MaxContentLength_1EventPerChunk(t *testing.T) {
	logs := testLogs()

	minLength, maxLength, events := jsonEncodeEventsBytes(logs, &Config{})

	numEvents := len(events)

	assert.True(t, numEvents > 1, "More than 1 event required")

	assert.True(t, minLength >= maxLength/2, "Smallest event >= half largest event required")

	// Setting chunk length to have 1 event per chunk.
	chunkLength := maxLength
	config := Config{MaxContentLength: chunkLength}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	chunkCh := logs.chunkEvents(ctx, zap.NewNop(), &config)

	numChunks := 0

	for chunk := range chunkCh {
		assert.Nil(t, chunk.err)
		assert.Len(t, chunk.buf.Bytes(), len(events[numChunks]))
		numChunks++
	}

	// Number of chunks should equal number of events.
	assert.Equal(t, numEvents, numChunks)
}

func Test_chunkEvents_MaxContentLength_1EventPerChunk_Cancel(t *testing.T) {
	logs := testLogs()

	min, max, events := jsonEncodeEventsBytes(logs, &Config{})

	assert.True(t, len(events) > 1, "More than 1 event required")

	assert.True(t, min >= max/2, "Smallest event >= half largest event required")

	// Setting chunk length to have as many chunks as there are events.
	chunkLength := max
	config := Config{MaxContentLength: chunkLength}

	ctx, cancel := context.WithCancel(context.Background())

	chunkCh := logs.chunkEvents(ctx, zap.NewNop(), &config)

	_, ok := <-chunkCh

	assert.True(t, ok, "Chunk channel open before cancel")

	cancel()

	// Giving time for chunkCh to close.
	time.Sleep(time.Millisecond)

	_, ok = <-chunkCh

	assert.True(t, !ok, "Chunk channel closed after cancel")

}

func Test_chunkEvents_MaxContentLength_2EventsPerChunk(t *testing.T) {
	logs := testLogs()

	min, max, events := jsonEncodeEventsBytes(logs, &Config{})

	numEvents := len(events)

	// Chunk max content length = 2 * max and this condition results in 2 event per chunk.
	assert.True(t, min >= max/2)

	chunkLength := 2 * max
	config := Config{MaxContentLength: chunkLength}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Config max content length equal to the length of the largest event.
	chunkCh := logs.chunkEvents(ctx, zap.NewNop(), &config)
	defer cancel()

	numChunks := 0

	for chunk := range chunkCh {
		assert.Nil(t, chunk.err)
		numChunks++
	}

	// 2 events per chunk.
	assert.Equal(t, numEvents, 2*numChunks)
}

func Test_chunkEvents_JSONEncodeError(t *testing.T) {
	logs := testLogs()

	// Setting a log logsIdx body to +Inf
	logs.ResourceLogs().At(0).
		InstrumentationLibraryLogs().At(0).
		Logs().At(0).
		Body().SetDoubleVal(math.Inf(1))

	// JSON Encoding +Inf should trigger unsupported value error
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	chunkCh := logs.chunkEvents(ctx, zap.NewNop(), &Config{})

	// chunk should contain an unsupported value error triggered by JSON Encoding +Inf
	chunk := <-chunkCh

	assert.Nil(t, chunk.buf)
	assert.Contains(t, chunk.err.Error(), "json: unsupported value: +Inf")

	// the error should cause the channel to be closed.
	_, ok := <-chunkCh
	assert.True(t, !ok, "Events channel should be closed on error")
}

func Test_chunkEvents_JSONEncodeError_Cancel(t *testing.T) {
	logs := testLogs()

	// Setting a log logsIdx body to +Inf
	logs.ResourceLogs().At(0).
		InstrumentationLibraryLogs().At(0).
		Logs().At(0).
		Body().SetDoubleVal(math.Inf(1))

	// JSON Encoding +Inf should trigger unsupported value error
	ctx, cancel := context.WithCancel(context.Background())

	chunkCh := logs.chunkEvents(ctx, zap.NewNop(), &Config{})

	cancel()

	// Giving time for chunkCh to close.
	time.Sleep(time.Millisecond)

	chunk, ok := <-chunkCh

	assert.Nil(t, chunk)
	assert.True(t, !ok, "Cancel should close events channel")
}

func makeLog(record pdata.LogRecord) pdata.Logs {
	logs := pdata.NewLogs()
	logs.ResourceLogs().Resize(1)
	rl := logs.ResourceLogs().At(0)
	rl.InstrumentationLibraryLogs().Resize(1)
	ill := rl.InstrumentationLibraryLogs().At(0)
	ill.Logs().Append(record)
	return logs
}

func commonLogSplunkEvent(
	event interface{},
	ts pdata.TimestampUnixNano,
	fields map[string]interface{},
	host string,
	source string,
	sourcetype string,
) *splunk.Event {
	return &splunk.Event{
		Time:       nanoTimestampToEpochMilliseconds(ts),
		Host:       host,
		Event:      event,
		Source:     source,
		SourceType: sourcetype,
		Fields:     fields,
	}
}

func Test_nilLogs(t *testing.T) {
	logs := logDataWrapper{&[]pdata.Logs{pdata.NewLogs()}[0]}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	chunkCh := logs.chunkEvents(ctx, zap.NewNop(), &Config{})
	chunk := <-chunkCh
	assert.Equal(t, 0, chunk.buf.Len())
}

func Test_nilResourceLogs(t *testing.T) {
	logs := logDataWrapper{&[]pdata.Logs{pdata.NewLogs()}[0]}
	logs.ResourceLogs().Resize(1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	chunkCh := logs.chunkEvents(ctx, zap.NewNop(), &Config{})
	chunk := <-chunkCh
	assert.Equal(t, 0, chunk.buf.Len())
}

func Test_nilInstrumentationLogs(t *testing.T) {
	logs := logDataWrapper{&[]pdata.Logs{pdata.NewLogs()}[0]}
	logs.ResourceLogs().Resize(1)
	resourceLog := logs.ResourceLogs().At(0)
	resourceLog.InstrumentationLibraryLogs().Resize(1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	chunkCh := logs.chunkEvents(ctx, zap.NewNop(), &Config{})
	chunk := <-chunkCh
	assert.Equal(t, 0, chunk.buf.Len())
}

func Test_nanoTimestampToEpochMilliseconds(t *testing.T) {
	splunkTs := nanoTimestampToEpochMilliseconds(1001000000)
	assert.Equal(t, 1.001, *splunkTs)
	splunkTs = nanoTimestampToEpochMilliseconds(1001990000)
	assert.Equal(t, 1.002, *splunkTs)
	splunkTs = nanoTimestampToEpochMilliseconds(0)
	assert.True(t, nil == splunkTs)
}

func Test_numLogs(t *testing.T) {
	// See nested structure of logs in testLogs() comments.
	logs := testLogs()

	_0_0_0 := &logIndex{origIdx: 0, instIdx: 0, logsIdx: 0}
	got := logs.numLogs(_0_0_0)

	assert.Equal(t, 6, got)

	_0_1_1 := &logIndex{origIdx: 0, instIdx: 1, logsIdx: 1}
	got = logs.numLogs(_0_1_1)

	assert.Equal(t, 4, got)
}

func Test_subLogs(t *testing.T) {
	// See nested structure of logs in testLogs() comments.
	logs := testLogs()

	// Logs subset from leftmost index.
	_0_0_0 := &logIndex{origIdx: 0, instIdx: 0, logsIdx: 0}
	got := logDataWrapper{logs.subLogs(_0_0_0)}

	assert.Equal(t, 6, got.numLogs(_0_0_0))

	orig := *got.InternalRep().Orig

	assert.Equal(t, "(0, 0, 0)", orig[0].InstrumentationLibraryLogs[0].Logs[0].Name)
	assert.Equal(t, "(0, 1, 0)", orig[0].InstrumentationLibraryLogs[1].Logs[0].Name)
	assert.Equal(t, "(0, 1, 1)", orig[0].InstrumentationLibraryLogs[1].Logs[1].Name)
	assert.Equal(t, "(1, 0, 0)", orig[1].InstrumentationLibraryLogs[0].Logs[0].Name)
	assert.Equal(t, "(1, 1, 0)", orig[1].InstrumentationLibraryLogs[1].Logs[0].Name)
	assert.Equal(t, "(1, 2, 0)", orig[1].InstrumentationLibraryLogs[2].Logs[0].Name)

	// Logs subset from rightmost index.
	_1_2_0 := &logIndex{origIdx: 1, instIdx: 2, logsIdx: 0}
	got = logDataWrapper{logs.subLogs(_1_2_0)}

	assert.Equal(t, 1, got.numLogs(_0_0_0))

	orig = *got.InternalRep().Orig

	assert.Equal(t, "(1, 2, 0)", orig[0].InstrumentationLibraryLogs[0].Logs[0].Name)

	// Logs subset from an in-between index.
	_1_1_0 := &logIndex{origIdx: 1, instIdx: 1, logsIdx: 0}
	got = logDataWrapper{logs.subLogs(_1_1_0)}

	assert.Equal(t, 2, got.numLogs(_0_0_0))

	orig = *got.InternalRep().Orig

	assert.Equal(t, "(1, 1, 0)", orig[0].InstrumentationLibraryLogs[0].Logs[0].Name)
	assert.Equal(t, "(1, 2, 0)", orig[0].InstrumentationLibraryLogs[1].Logs[0].Name)
}

// Creates pdata.Logs for testing.
//
// Structure of the pdata.Logs created showing indices:
//
//     0            1      <- orig index
//    / \         / | \
//   0   1      0  1  2    <- InstrumentationLibraryLogs index
//  /   / \    /  /  /
// 0   0   1  0  0  0      <- Logs index
//
// The log records are named in the pattern:
// (<orig index>, <InstrumentationLibraryLogs index>, <Logs index>)
//
// The log records are about the same size and some test depend this fact.
//
func testLogs() *logDataWrapper {
	logs := logDataWrapper{&[]pdata.Logs{pdata.NewLogs()}[0]}
	logs.ResourceLogs().Resize(2)

	rl0 := logs.ResourceLogs().At(0)
	rl0.InstrumentationLibraryLogs().Resize(2)

	log := pdata.NewLogRecord()
	log.SetName("(0, 0, 0)")
	rl0.InstrumentationLibraryLogs().At(0).Logs().Append(log)

	log = pdata.NewLogRecord()
	log.SetName("(0, 1, 0)")
	rl0.InstrumentationLibraryLogs().At(1).Logs().Append(log)

	log = pdata.NewLogRecord()
	log.SetName("(0, 1, 1)")
	rl0.InstrumentationLibraryLogs().At(1).Logs().Append(log)

	rl1 := logs.ResourceLogs().At(1)
	rl1.InstrumentationLibraryLogs().Resize(3)

	log = pdata.NewLogRecord()
	log.SetName("(1, 0, 0)")
	rl1.InstrumentationLibraryLogs().At(0).Logs().Append(log)

	log = pdata.NewLogRecord()
	log.SetName("(1, 1, 0)")
	rl1.InstrumentationLibraryLogs().At(1).Logs().Append(log)

	log = pdata.NewLogRecord()
	log.SetName("(1, 2, 0)")
	rl1.InstrumentationLibraryLogs().At(2).Logs().Append(log)

	return &logs
}

func jsonEncodeEventsBytes(logs *logDataWrapper, config *Config) (int, int, [][]byte) {
	events := make([][]byte, 0)
	// min, max number of bytes of smallest, largest events.
	var min, max int

	event := new(bytes.Buffer)
	encoder := json.NewEncoder(event)

	rl := logs.ResourceLogs()
	for i := 0; i < rl.Len(); i++ {
		ill := rl.At(i).InstrumentationLibraryLogs()
		for j := 0; j < ill.Len(); j++ {
			l := ill.At(j).Logs()
			for k := 0; k < l.Len(); k++ {
				if err := encoder.Encode(mapLogRecordToSplunkEvent(l.At(k), config, zap.NewNop())); err == nil {
					event.WriteString("\r\n\r\n")
					dst := make([]byte, len(event.Bytes()))
					copy(dst, event.Bytes())
					events = append(events, dst)
					if event.Len() < min || min == 0 {
						min = event.Len()
					}
					if event.Len() > max {
						max = event.Len()
					}
					event.Reset()
				}
			}
		}
	}
	return min, max, events
}
