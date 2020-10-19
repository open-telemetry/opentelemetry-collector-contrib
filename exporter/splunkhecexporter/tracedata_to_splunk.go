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

	"github.com/open-telemetry/opentelemetry-collector-contrib/internal/splunk"
)

func traceDataToSplunk(logger *zap.Logger, data pdata.Traces, config *Config) ([]*splunk.Event, int) {
	// TODO replace with span JSON object.
	internalData := internaldata.TraceDataToOC(data)
	numDroppedSpans := 0
	splunkEvents := make([]*splunk.Event, 0, data.SpanCount())
	for i := 0; i < data.ResourceSpans().Len(); i++ {
		rs := data.ResourceSpans().At(i)
		host := unknownHostName
		source := config.Source
		sourceType := config.SourceType
		if !rs.Resource().IsNil() {
			if conventionHost, isSet := rs.Resource().Attributes().Get(conventions.AttributeHostHostname); isSet {
				host = conventionHost.StringVal()
			}
			if sourceSet, isSet := rs.Resource().Attributes().Get(conventions.AttributeServiceName); isSet {
				source = sourceSet.StringVal()
			}
			if sourcetypeSet, isSet := rs.Resource().Attributes().Get(splunk.SourcetypeLabel); isSet {
				sourceType = sourcetypeSet.StringVal()
			}
		}
		for sils := 0; sils < rs.InstrumentationLibrarySpans().Len(); sils++ {
			ils := rs.InstrumentationLibrarySpans().At(sils)
			for si := 0; si < ils.Spans().Len(); si++ {
				span := ils.Spans().At(si)
				spanOC := internalData[i].Spans[(sils+1)*si]

				se := &splunk.Event{
					Time:       timestampToEpochMilliseconds(span.StartTime()),
					Host:       host,
					Source:     source,
					SourceType: sourceType,
					Index:      config.Index,
					Event:      spanOC,
				}
				splunkEvents = append(splunkEvents, se)
			}
		}
	}

	return splunkEvents, numDroppedSpans
}
