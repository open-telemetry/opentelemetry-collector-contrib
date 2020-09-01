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
	"math"

	"go.opentelemetry.io/collector/consumer/pdata"
	"go.uber.org/zap"
)

const (
	sourceLabel     = "service.name"
	sourcetypeLabel = "source.type"
)

func logDataToSplunk(logger *zap.Logger, ld pdata.Logs, config *Config) ([]*splunkEvent, int) {
	numDroppedLogs := 0
	splunkEvents := make([]*splunkEvent, 0)
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		rl := rls.At(i)
		if rl.IsNil() {
			continue
		}

		ills := rl.InstrumentationLibraryLogs()
		for j := 0; j < ills.Len(); j++ {
			ils := ills.At(j)
			if ils.IsNil() {
				continue
			}

			logs := ils.Logs()
			for j := 0; j < logs.Len(); j++ {
				lr := logs.At(j)
				if lr.IsNil() {
					numDroppedLogs++
					continue
				}
				ev := mapLogRecordToSplunkEvent(lr, config)
				splunkEvents = append(splunkEvents, ev)
			}
		}
	}

	return splunkEvents, numDroppedLogs
}

func mapLogRecordToSplunkEvent(lr pdata.LogRecord, config *Config) *splunkEvent {
	var host string
	var source string
	var sourcetype string
	fields := map[string]string{}
	lr.Attributes().ForEach(func(k string, v pdata.AttributeValue) {
		if k == hostnameLabel {
			host = v.StringVal()
		} else if k == sourceLabel {
			source = v.StringVal()
		} else if k == sourcetypeLabel {
			sourcetype = v.StringVal()
		} else {
			fields[k] = v.StringVal()
		}
	})

	if host == "" {
		host = unknownHostName
	}

	if source == "" {
		source = config.Source
	}

	if sourcetype == "" {
		sourcetype = config.SourceType
	}

	return &splunkEvent{
		Time:       nanoTimestampToEpochMilliseconds(lr.Timestamp()),
		Host:       host,
		Source:     source,
		SourceType: sourcetype,
		Index:      config.Index,
		Event:      lr.Body().StringVal(),
		Fields:     fields,
	}
}

func nanoTimestampToEpochMilliseconds(ts pdata.TimestampUnixNano) float64 {
	return float64(ts/1e9) + math.Round(float64(ts)/1e6)/1e3
}
