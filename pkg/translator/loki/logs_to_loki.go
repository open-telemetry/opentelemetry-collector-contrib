// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package loki // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/loki"

import (
	"fmt"

	"github.com/grafana/loki/pkg/logproto"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

type PushRequest struct {
	*logproto.PushRequest
	Report *PushReport
}

// PushReport contains the summary for the outcome of a LogsToLoki operation
type PushReport struct {
	Errors       []error
	NumSubmitted int
	NumDropped   int
}

// LogsToLokiRequests converts a Logs pipeline data into Loki PushRequests grouped
// by tenant. The tenant value is inferred from the `loki.tenant` resource or log
// attribute hint. If the `loki.tenant` attribute is present in both resource or
// log attributes, then the resource attribute takes precedence.
// Labels for each record are inferred based on the hints "loki.attribute.labels"
// and "loki.resource.labels". Each hint might contain a comma-separated list of
// attributes (resource or record) that should be promoted to a Loki label. Those
// attributes are removed from the body as a result, otherwise they would be shown
// in duplicity in Loki.
// PushStreams are created based on the labels: all records containing the same
// set of labels are part of the same stream. All streams are then packed within
// the resulting PushRequest.
// When this function isn't able to marshal a log record, the log record is dropped
// and processing continues, so that the caller can decide to either skip the entire
// batch or send only the data that could be parsed. The caller can use the PushReport
// to make this decision, as it includes all of the errors that were encountered,
// as well as the number of items dropped and submitted.
func LogsToLokiRequests(ld plog.Logs) map[string]PushRequest {
	groups := map[string]pushRequestGroup{}

	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		ills := rls.At(i).ScopeLogs()

		for j := 0; j < ills.Len(); j++ {
			logs := ills.At(j).LogRecords()
			for k := 0; k < logs.Len(); k++ {

				// similarly, we may remove attributes, so change only our version
				log := plog.NewLogRecord()
				logs.At(k).CopyTo(log)

				// we may remove attributes, so we make a copy and change our version
				resource := pcommon.NewResource()
				rls.At(i).Resource().CopyTo(resource)

				// resolve tenant and get/create a push request group
				tenant := getTenantFromTenantHint(log.Attributes(), resource.Attributes())
				group, ok := groups[tenant]
				if !ok {
					group = pushRequestGroup{
						report:  &PushReport{},
						streams: make(map[string]*logproto.Stream),
					}
					groups[tenant] = group
				}

				format := getFormatFromFormatHint(log.Attributes(), resource.Attributes())

				mergedLabels := convertAttributesAndMerge(log.Attributes(), resource.Attributes())
				// remove the attributes that were promoted to labels
				removeAttributes(log.Attributes(), mergedLabels)
				removeAttributes(resource.Attributes(), mergedLabels)

				// create the stream name based on the labels
				labels := mergedLabels.String()
				entry, err := convertLogToLokiEntry(log, resource, format)
				if err != nil {
					// Couldn't convert so dropping log.
					group.report.Errors = append(group.report.Errors, fmt.Errorf("failed to convert, dropping log: %w", err))
					group.report.NumDropped++
					continue
				}

				group.report.NumSubmitted++

				if stream, ok := group.streams[labels]; ok {
					stream.Entries = append(stream.Entries, *entry)
					continue
				}

				group.streams[labels] = &logproto.Stream{
					Labels:  labels,
					Entries: []logproto.Entry{*entry},
				}
			}
		}
	}

	requests := make(map[string]PushRequest)
	for tenant, g := range groups {
		pr := &logproto.PushRequest{
			Streams: make([]logproto.Stream, len(g.streams)),
		}

		i := 0
		for _, stream := range g.streams {
			pr.Streams[i] = *stream
			i++
		}
		requests[tenant] = PushRequest{
			PushRequest: pr,
			Report:      g.report,
		}
	}
	return requests
}

func getFormatFromFormatHint(logAttr pcommon.Map, resourceAttr pcommon.Map) string {
	format := formatJSON
	formatVal, found := resourceAttr.Get(hintFormat)
	if !found {
		formatVal, found = logAttr.Get(hintFormat)
	}

	if found {
		format = formatVal.AsString()
	}
	return format
}

// getTenantFromTenantHint extract an attribute based on the tenant hint.
// it looks up for the attribute first in resource attributes and fallbacks to
// record attributes if it is not found.
func getTenantFromTenantHint(logAttr pcommon.Map, resourceAttr pcommon.Map) string {
	var tenant string
	hintAttr, found := resourceAttr.Get(hintTenant)
	if !found {
		if hintAttr, found = logAttr.Get(hintTenant); !found {
			return tenant
		}
	}

	if tenantAttr, found := resourceAttr.Get(hintAttr.Str()); found {
		tenant = tenantAttr.Str()
	} else {
		if tenantAttr, found = logAttr.Get(hintAttr.Str()); found {
			tenant = tenantAttr.Str()
		}
	}
	return tenant
}

type pushRequestGroup struct {
	streams map[string]*logproto.Stream
	report  *PushReport
}

// LogsToLoki converts a Logs pipeline data into a Loki PushRequest.
// Labels for each record are inferred based on the hints "loki.attribute.labels"
// and "loki.resource.labels". Each hint might contain a comma-separated list of
// attributes (resource or record) that should be promoted to a Loki label. Those
// attributes are removed from the body as a result, otherwise they would be shown
// in duplicity in Loki.
// PushStreams are created based on the labels: all records containing the same
// set of labels are part of the same stream. All streams are then packed within
// the resulting PushRequest.
// When this function isn't able to marshal a log record, the log record is dropped
// and processing continues, so that the caller can decide to either skip the entire
// batch or send only the data that could be parsed. The caller can use the PushReport
// to make this decision, as it includes all of the errors that were encountered,
// as well as the number of items dropped and submitted.
// Deprecated: [v0.62.0] will be removed after v0.63.0. Use LogsToLokiRequests instead.
func LogsToLoki(ld plog.Logs) (*logproto.PushRequest, *PushReport) {
	report := &PushReport{}

	streams := make(map[string]*logproto.Stream)
	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		ills := rls.At(i).ScopeLogs()

		for j := 0; j < ills.Len(); j++ {
			logs := ills.At(j).LogRecords()
			for k := 0; k < logs.Len(); k++ {

				// similarly, we may remove attributes, so change only our version
				log := plog.NewLogRecord()
				logs.At(k).CopyTo(log)

				// we may remove attributes, so we make a copy and change our version
				resource := pcommon.NewResource()
				rls.At(i).Resource().CopyTo(resource)

				format := getFormatFromFormatHint(log.Attributes(), resource.Attributes())

				mergedLabels := convertAttributesAndMerge(log.Attributes(), resource.Attributes())
				// remove the attributes that were promoted to labels
				removeAttributes(log.Attributes(), mergedLabels)
				removeAttributes(resource.Attributes(), mergedLabels)

				// create the stream name based on the labels
				labels := mergedLabels.String()

				entry, err := convertLogToLokiEntry(log, resource, format)
				if err != nil {
					// Couldn't convert so dropping log.
					report.Errors = append(report.Errors, fmt.Errorf("failed to convert, dropping log: %w", err))
					report.NumDropped++
					continue
				}

				report.NumSubmitted++

				if stream, ok := streams[labels]; ok {
					stream.Entries = append(stream.Entries, *entry)
					continue
				}

				streams[labels] = &logproto.Stream{
					Labels:  labels,
					Entries: []logproto.Entry{*entry},
				}
			}
		}
	}

	pr := &logproto.PushRequest{
		Streams: make([]logproto.Stream, len(streams)),
	}

	i := 0
	for _, stream := range streams {
		pr.Streams[i] = *stream
		i++
	}

	return pr, report
}
