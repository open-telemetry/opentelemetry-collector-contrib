// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loki // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/loki"

import (
	"fmt"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"

	prometheustranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/prometheus"
)

type PushRequest struct {
	*push.PushRequest
	Report *PushReport
}

// PushReport contains the summary for the outcome of a LogsToLoki operation
type PushReport struct {
	Errors       []error
	NumSubmitted int
	NumDropped   int
}

const (
	levelAttributeName = "level"
)

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
func LogsToLokiRequests(ld plog.Logs, defaultLabelsEnabled map[string]bool) map[string]PushRequest {
	groups := map[string]pushRequestGroup{}

	rls := ld.ResourceLogs()
	for i := 0; i < rls.Len(); i++ {
		ills := rls.At(i).ScopeLogs()
		resource := rls.At(i).Resource()

		for j := 0; j < ills.Len(); j++ {
			logs := ills.At(j).LogRecords()
			scope := ills.At(j).Scope()
			for k := 0; k < logs.Len(); k++ {
				log := logs.At(k)
				tenant := GetTenantFromTenantHint(log.Attributes(), resource.Attributes())
				group, ok := groups[tenant]
				if !ok {
					group = pushRequestGroup{
						report:  &PushReport{},
						streams: make(map[string]*push.Stream),
					}
					groups[tenant] = group
				}

				entry, err := LogToLokiEntry(log, resource, scope, defaultLabelsEnabled)
				if err != nil {
					// Couldn't convert so dropping log.
					group.report.Errors = append(group.report.Errors, fmt.Errorf("failed to convert, dropping log: %w", err))
					group.report.NumDropped++
					continue
				}

				group.report.NumSubmitted++

				// create the stream name based on the labels
				labels := entry.Labels.String()
				if stream, ok := group.streams[labels]; ok {
					stream.Entries = append(stream.Entries, *entry.Entry)
					continue
				}

				group.streams[labels] = &push.Stream{
					Labels:  labels,
					Entries: []push.Entry{*entry.Entry},
				}
			}
		}
	}

	requests := make(map[string]PushRequest)
	for tenant, g := range groups {
		pr := &push.PushRequest{
			Streams: make([]push.Stream, len(g.streams)),
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

// PushEntry is Loki log entry enriched with labels
type PushEntry struct {
	Entry  *push.Entry
	Labels model.LabelSet
}

// LogToLokiEntry converts LogRecord into Loki log entry enriched with normalized labels
func LogToLokiEntry(lr plog.LogRecord, rl pcommon.Resource, scope pcommon.InstrumentationScope, defaultLabelsEnabled map[string]bool) (*PushEntry, error) {
	// we may remove attributes, so change only our version
	log := plog.NewLogRecord()
	lr.CopyTo(log)

	// similarly, we may remove attributes, so we make a copy and change our version
	resource := pcommon.NewResource()
	rl.CopyTo(resource)

	if enabled, ok := defaultLabelsEnabled[levelLabel]; !ok || enabled {
		// adds level attribute from log.severityNumber
		addLogLevelAttributeAndHint(log)
	}

	format := getFormatFromFormatHint(log.Attributes(), resource.Attributes())

	mergedLabels := convertAttributesAndMerge(log.Attributes(), resource.Attributes(), defaultLabelsEnabled)
	// remove the attributes that were promoted to labels
	removeAttributes(log.Attributes(), mergedLabels)
	removeAttributes(resource.Attributes(), mergedLabels)

	entry, err := convertLogToLokiEntry(log, resource, format, scope)
	if err != nil {
		return nil, err
	}

	labels := model.LabelSet{}
	for label := range mergedLabels {
		// Loki doesn't support dots in label names
		// labelName is normalized label name to follow Prometheus label names standard
		labelName := prometheustranslator.NormalizeLabel(string(label))
		labels[model.LabelName(labelName)] = mergedLabels[label]
	}

	return &PushEntry{
		Entry:  entry,
		Labels: labels,
	}, nil
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

// GetTenantFromTenantHint extract an attribute based on the tenant hint.
// it looks up for the attribute first in resource attributes and fallbacks to
// record attributes if it is not found.
func GetTenantFromTenantHint(logAttr pcommon.Map, resourceAttr pcommon.Map) string {
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
	streams map[string]*push.Stream
	report  *PushReport
}

func addLogLevelAttributeAndHint(log plog.LogRecord) {
	if log.SeverityNumber() == plog.SeverityNumberUnspecified {
		return
	}
	addHint(log)
	if _, found := log.Attributes().Get(levelAttributeName); !found {
		level := severityNumberToLevel[log.SeverityNumber().String()]
		log.Attributes().PutStr(levelAttributeName, level)
	}
}

func addHint(log plog.LogRecord) {
	if value, found := log.Attributes().Get(hintAttributes); found {
		switch value.Type() {
		case pcommon.ValueTypeSlice:
			value.Slice().AppendEmpty().SetStr(levelAttributeName)
		case pcommon.ValueTypeStr:
			log.Attributes().PutStr(hintAttributes, fmt.Sprintf("%s,%s", value.AsString(), levelAttributeName))
		}
	} else {
		log.Attributes().PutStr(hintAttributes, levelAttributeName)
	}
}

var severityNumberToLevel = map[string]string{
	plog.SeverityNumberUnspecified.String(): "UNSPECIFIED",
	plog.SeverityNumberTrace.String():       "TRACE",
	plog.SeverityNumberTrace2.String():      "TRACE2",
	plog.SeverityNumberTrace3.String():      "TRACE3",
	plog.SeverityNumberTrace4.String():      "TRACE4",
	plog.SeverityNumberDebug.String():       "DEBUG",
	plog.SeverityNumberDebug2.String():      "DEBUG2",
	plog.SeverityNumberDebug3.String():      "DEBUG3",
	plog.SeverityNumberDebug4.String():      "DEBUG4",
	plog.SeverityNumberInfo.String():        "INFO",
	plog.SeverityNumberInfo2.String():       "INFO2",
	plog.SeverityNumberInfo3.String():       "INFO3",
	plog.SeverityNumberInfo4.String():       "INFO4",
	plog.SeverityNumberWarn.String():        "WARN",
	plog.SeverityNumberWarn2.String():       "WARN2",
	plog.SeverityNumberWarn3.String():       "WARN3",
	plog.SeverityNumberWarn4.String():       "WARN4",
	plog.SeverityNumberError.String():       "ERROR",
	plog.SeverityNumberError2.String():      "ERROR2",
	plog.SeverityNumberError3.String():      "ERROR3",
	plog.SeverityNumberError4.String():      "ERROR4",
	plog.SeverityNumberFatal.String():       "FATAL",
	plog.SeverityNumberFatal2.String():      "FATAL2",
	plog.SeverityNumberFatal3.String():      "FATAL3",
	plog.SeverityNumberFatal4.String():      "FATAL4",
}
