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
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/translator/conventions"
	"go.opentelemetry.io/collector/translator/internaldata"
	"go.uber.org/zap"
)

type splunkEvent struct {
	Time       float64           `json:"time"`                 // epoch time
	Host       string            `json:"host"`                 // hostname
	Source     string            `json:"source,omitempty"`     // optional description of the source of the event; typically the app's name
	SourceType string            `json:"sourcetype,omitempty"` // optional name of a Splunk parsing configuration; this is usually inferred by Splunk
	Index      string            `json:"index,omitempty"`      // optional name of the Splunk index to store the event in; not required if the token has a default index set in Splunk
	Event      interface{}       `json:"event"`                // Payload of the event.
	Fields     map[string]string `json:"fields,omitempty"`     // Fields of the event.
}

func traceDataToSplunk(logger *zap.Logger, data pdata.Traces, config *Config) ([]*splunkEvent, int) {
	octds := internaldata.TraceDataToOC(data)
	numDroppedSpans := 0
	splunkEvents := make([]*splunkEvent, 0, data.SpanCount())
	for _, octd := range octds {
		var host string
		if octd.Resource != nil {
			host = octd.Resource.Labels[conventions.AttributeHostHostname]
		}
		if host == "" {
			host = unknownHostName
		}
		for _, span := range octd.Spans {
			if span.StartTime == nil {
				logger.Debug(
					"Span dropped as it had no start timestamp",
					zap.Any("span", span))
				numDroppedSpans++
				continue
			}
			se := &splunkEvent{
				Time:       timestampToEpochMilliseconds(span.StartTime),
				Host:       host,
				Source:     config.Source,
				SourceType: config.SourceType,
				Index:      config.Index,
				Event:      span,
			}
			splunkEvents = append(splunkEvents, se)
		}
	}

	return splunkEvents, numDroppedSpans
}
