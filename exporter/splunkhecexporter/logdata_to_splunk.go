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

// eventsBuf is a buffer of JSON encoded Splunk events.
// The events are created from LogRecord(s) where one event maps to one LogRecord.
type eventsBuf struct {
	buf *bytes.Buffer
	// index is the eventIndex of the 1st event in buf.
	index *eventIndex
	err   error
}

// The index of an event composed of indices of the event's LogRecord.
type eventIndex struct {
	// Index of the LogRecord slice element from which the event is created.
	log int
	// Index of the InstrumentationLibraryLogs slice element parent of the LogRecord.
	lib int
	// Index of the ResourceLogs slice element parent of the InstrumentationLibraryLogs.
	src int
}

type logDataWrapper struct {
	*pdata.Logs
}

func (ld *logDataWrapper) eventsInChunks(logger *zap.Logger, config *Config) (chan *eventsBuf, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	eventsCh := make(chan *eventsBuf)

	go func() {
		defer close(eventsCh)

		// event buffers a single event.
		event := new(bytes.Buffer)
		encoder := json.NewEncoder(event)

		// events buffers events up to the max content length.
		events := &eventsBuf{buf: new(bytes.Buffer)}

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
							eventsCh <- &eventsBuf{buf: nil, index: nil, err: consumererror.Permanent(err)}
							return
						}
						event.WriteString("\r\n\r\n")

						// The size of an event must be less than or equal to max content length.
						if config.MaxContentLength > 0 && event.Len() > config.MaxContentLength {
							err := fmt.Errorf("found a log event bigger than max content length (event: %d bytes, max: %d bytes)", config.MaxContentLength, event.Len())
							eventsCh <- &eventsBuf{buf: nil, index: nil, err: consumererror.Permanent(err)}
							return
						}

						// Moving the event to events.buf if length will be <= max content length.
						// Max content length <= 0 is interpreted as unbound.
						if events.buf.Len()+event.Len() <= config.MaxContentLength || config.MaxContentLength <= 0 {
							// WriteTo() empties and resets buffer event.
							if _, err := event.WriteTo(events.buf); err != nil {
								eventsCh <- &eventsBuf{buf: nil, index: nil, err: consumererror.Permanent(err)}
								return
							}

							// Setting events index using the log record indices of the 1st event.
							if events.index == nil {
								events.index = &eventIndex{src: i, lib: j, log: k}
							}

							continue
						}

						eventsCh <- events

						// Creating a new events buffer.
						events = &eventsBuf{buf: new(bytes.Buffer)}
						// Setting events index using the log record indices of any current leftover event.
						if event.Len() != 0 {
							events.index = &eventIndex{src: i, lib: j, log: k}
						}
					}
				}
			}
		}

		// Writing any leftover event to eventsBuf buffer `events.buf`.
		if _, err := event.WriteTo(events.buf); err != nil {
			eventsCh <- &eventsBuf{buf: nil, index: nil, err: consumererror.Permanent(err)}
			return
		}

		eventsCh <- events

	}()
	return eventsCh, cancel
}

func (ld *logDataWrapper) countLogs(start *eventIndex) int {
	count, orig := 0, *ld.InternalRep().Orig
	for i := start.src; i < len(orig); i++ {
		for j, iLLogs := range orig[i].InstrumentationLibraryLogs {
			switch {
			case i == start.src && j < start.lib:
				continue
			default:
				count += len(iLLogs.Logs)
			}
		}
	}
	return count - start.log
}

func (ld *logDataWrapper) trimLeft(end *eventIndex) *pdata.Logs {
	clone := ld.Clone()
	orig := *clone.InternalRep().Orig
	orig = orig[end.src:]
	orig[end.src].InstrumentationLibraryLogs = orig[end.src].InstrumentationLibraryLogs[end.lib:]
	orig[end.src].InstrumentationLibraryLogs[end.lib].Logs = orig[end.src].InstrumentationLibraryLogs[end.lib].Logs[end.log:]
	return &clone
}

func (ld *logDataWrapper) processErr(index *eventIndex, err error) (int, error) {
	if consumererror.IsPermanent(err) {
		return ld.countLogs(index), err
	}

	if _, ok := err.(consumererror.PartialError); ok {
		failedLogs := ld.trimLeft(index)
		return failedLogs.LogRecordCount(), consumererror.PartialLogsError(err, *failedLogs)
	}
	return ld.LogRecordCount(), err
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
