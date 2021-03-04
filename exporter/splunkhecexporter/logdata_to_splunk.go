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
	"fmt"
	"time"

	"go.opentelemetry.io/collector/consumer/consumererror"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.uber.org/zap"

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

// eventsChunk buffers JSON encoded Splunk events.
// The events are created from LogRecord(s) where one event is created from one LogRecord.
type eventsChunk struct {
	buf *bytes.Buffer
	// The logIndex of the LogRecord of 1st event in buf.
	index *logIndex
	err   error
}

// Composite index of a log record in pdata.Logs.
type logIndex struct {
	// Index in orig list (i.e. root parent index).
	origIdx int
	// Index in InstrumentationLibraryLogs list (i.e. immediate parent index).
	instIdx int
	// Index in Logs list (i.e. the log record index).
	logsIdx int
}

type logDataWrapper struct {
	*pdata.Logs
}

func (ld *logDataWrapper) chunkEvents(logger *zap.Logger, config *Config) (chan *eventsChunk, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	chunkCh := make(chan *eventsChunk)

	go func() {
		defer close(chunkCh)

		// event buffers a single event.
		event := new(bytes.Buffer)
		encoder := json.NewEncoder(event)

		// chunk buffers events up to the max content length.
		chunk := &eventsChunk{buf: new(bytes.Buffer)}

		rl := ld.ResourceLogs()
		for i := 0; i < rl.Len(); i++ {
			ill := rl.At(i).InstrumentationLibraryLogs()
			for j := 0; j < ill.Len(); j++ {
				l := ill.At(j).Logs()
				for k := 0; k < l.Len(); k++ {
					select {
					case <-ctx.Done():
						return
					default:
						if err := encoder.Encode(mapLogRecordToSplunkEvent(l.At(k), config, logger)); err != nil {
							chunkCh <- &eventsChunk{buf: nil, index: nil, err: consumererror.Permanent(err)}
							return
						}
						event.WriteString("\r\n\r\n")

					addToChunk:
						// The size of an event must be less than or equal to max content length.
						if config.MaxContentLength > 0 && event.Len() > config.MaxContentLength {
							chunkCh <- &eventsChunk{
								buf:   nil,
								index: nil,
								err:   consumererror.Permanent(fmt.Errorf("log event bytes exceed max content length configured (log: %d, max: %d)", event.Len(), config.MaxContentLength)),
							}
							return
						}

						// Moving the event to chunk.buf if length will be <= max content length.
						// Max content length <= 0 is interpreted as unbound.
						if chunk.buf.Len()+event.Len() <= config.MaxContentLength || config.MaxContentLength <= 0 {
							// WriteTo() empties and resets buffer event.
							if _, err := event.WriteTo(chunk.buf); err != nil {
								chunkCh <- &eventsChunk{buf: nil, index: nil, err: consumererror.Permanent(err)}
								return
							}

							// Setting chunk index using the log logsIdx indices of the 1st event.
							if chunk.index == nil {
								chunk.index = &logIndex{origIdx: i, instIdx: j, logsIdx: k}
							}

							continue
						}

						chunkCh <- chunk

						// Creating a new events buffer.
						chunk = &eventsChunk{buf: new(bytes.Buffer)}

						// Adding remaining event to the new chunk
						if event.Len() != 0 {
							goto addToChunk
						}
					}
				}
			}
		}

		chunkCh <- chunk
	}()

	return chunkCh, cancel
}

func (ld *logDataWrapper) numLogs(from *logIndex) int {
	count, orig := 0, *ld.InternalRep().Orig

	// Validating logIndex. Invalid index will cause out of range panic.
	_ = orig[from.origIdx].InstrumentationLibraryLogs[from.instIdx].Logs[from.logsIdx]

	for i := from.origIdx; i < len(orig); i++ {
		for j, library := range orig[i].InstrumentationLibraryLogs {
			switch {
			case i == from.origIdx && j < from.instIdx:
				continue
			default:
				count += len(library.Logs)
			}
		}
	}

	return count - from.logsIdx
}

func (ld *logDataWrapper) subLogs(from *logIndex) *pdata.Logs {
	clone := ld.Clone().InternalRep()

	subset := *clone.Orig
	subset = subset[from.origIdx:]
	subset[0].InstrumentationLibraryLogs = subset[0].InstrumentationLibraryLogs[from.instIdx:]
	subset[0].InstrumentationLibraryLogs[0].Logs = subset[0].InstrumentationLibraryLogs[0].Logs[from.logsIdx:]

	clone.Orig = &subset
	subsetLogs := pdata.LogsFromInternalRep(clone)

	return &subsetLogs
}

func mapLogRecordToSplunkEvent(lr pdata.LogRecord, config *Config, logger *zap.Logger) *splunk.Event {
	host := unknownHostName
	source := config.Source
	sourcetype := config.SourceType
	index := config.Index
	fields := map[string]interface{}{}
	lr.Attributes().ForEach(func(k string, v pdata.AttributeValue) {
		if k == conventions.AttributeHostName {
			host = v.StringVal()
		} else if k == conventions.AttributeServiceName {
			source = v.StringVal()
		} else if k == splunk.SourcetypeLabel {
			sourcetype = v.StringVal()
		} else if k == splunk.IndexLabel {
			index = v.StringVal()
		} else {
			fields[k] = convertAttributeValue(v, logger)
		}
	})

	eventValue := convertAttributeValue(lr.Body(), logger)
	return &splunk.Event{
		Time:       nanoTimestampToEpochMilliseconds(lr.Timestamp()),
		Host:       host,
		Source:     source,
		SourceType: sourcetype,
		Index:      index,
		Event:      eventValue,
		Fields:     fields,
	}
}

func convertAttributeValue(value pdata.AttributeValue, logger *zap.Logger) interface{} {
	switch value.Type() {
	case pdata.AttributeValueINT:
		return value.IntVal()
	case pdata.AttributeValueBOOL:
		return value.BoolVal()
	case pdata.AttributeValueDOUBLE:
		return value.DoubleVal()
	case pdata.AttributeValueSTRING:
		return value.StringVal()
	case pdata.AttributeValueMAP:
		values := map[string]interface{}{}
		value.MapVal().ForEach(func(k string, v pdata.AttributeValue) {
			values[k] = convertAttributeValue(v, logger)
		})
		return values
	case pdata.AttributeValueARRAY:
		arrayVal := value.ArrayVal()
		values := make([]interface{}, arrayVal.Len())
		for i := 0; i < arrayVal.Len(); i++ {
			values[i] = convertAttributeValue(arrayVal.At(i), logger)
		}
		return values
	case pdata.AttributeValueNULL:
		return nil
	default:
		logger.Debug("Unhandled value type", zap.String("type", value.Type().String()))
		return value
	}
}

// nanoTimestampToEpochMilliseconds transforms nanoseconds into <sec>.<ms>. For example, 1433188255.500 indicates 1433188255 seconds and 500 milliseconds after epoch.
func nanoTimestampToEpochMilliseconds(ts pdata.TimestampUnixNano) *float64 {
	duration := time.Duration(ts)
	if duration == 0 {
		// some telemetry sources send data with timestamps set to 0 by design, as their original target destinations
		// (i.e. before Open Telemetry) are setup with the know-how on how to consume them. In this case,
		// we want to omit the time field when sending data to the Splunk HEC so that the HEC adds a timestamp
		// at indexing time, which will be much more useful than a 0-epoch-time value.
		return nil
	}

	val := duration.Round(time.Millisecond).Seconds()
	return &val
}
