// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package loki // import "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/loki"

import (
	"fmt"
	"strings"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
)

const (
	hintAttributes = "loki.attribute.labels"
	hintResources  = "loki.resource.labels"
	hintTenant     = "loki.tenant"
	hintFormat     = "loki.format"
)

const (
	formatJSON   string = "json"
	formatLogfmt string = "logfmt"
	formatRaw    string = "raw"
)

func convertAttributesAndMerge(logAttrs pcommon.Map, resAttrs pcommon.Map) model.LabelSet {
	out := model.LabelSet{"exporter": "OTLP"}

	// Map service.namespace + service.name to job
	if job, ok := extractJob(resAttrs); ok {
		out[model.JobLabel] = model.LabelValue(job)
	}
	// Map service.instance.id to instance
	if instance, ok := extractInstance(resAttrs); ok {
		out[model.InstanceLabel] = model.LabelValue(instance)
	}

	if resourcesToLabel, found := resAttrs.Get(hintResources); found {
		labels := convertAttributesToLabels(resAttrs, resourcesToLabel)
		out = out.Merge(labels)
	}

	// get the hint from the log attributes, not from the resource
	// the value can be a single resource name to use as label
	// or a slice of string values
	if resourcesToLabel, found := logAttrs.Get(hintResources); found {
		labels := convertAttributesToLabels(resAttrs, resourcesToLabel)
		out = out.Merge(labels)
	}

	if attributesToLabel, found := logAttrs.Get(hintAttributes); found {
		labels := convertAttributesToLabels(logAttrs, attributesToLabel)
		out = out.Merge(labels)
	}

	// get tenant hint from resource attributes, fallback to record attributes
	// if it is not found
	if resourcesToLabel, found := resAttrs.Get(hintTenant); !found {
		if attributesToLabel, found := logAttrs.Get(hintTenant); found {
			labels := convertAttributesToLabels(logAttrs, attributesToLabel)
			out = out.Merge(labels)
		}
	} else {
		labels := convertAttributesToLabels(resAttrs, resourcesToLabel)
		out = out.Merge(labels)
	}

	return out
}

func convertAttributesToLabels(attributes pcommon.Map, attrsToSelect pcommon.Value) model.LabelSet {
	out := model.LabelSet{}

	attrs := parseAttributeNames(attrsToSelect)
	for _, attr := range attrs {
		attr = strings.TrimSpace(attr)

		av, ok := attributes.Get(attr)
		if !ok {
			// couldn't find the attribute under the given name directly
			// perhaps it's a nested attribute?
			av, ok = getNestedAttribute(attr, attributes) // shadows the OK from above on purpose
		}

		if ok {
			out[model.LabelName(attr)] = model.LabelValue(av.AsString())
		}
	}

	return out
}

func getNestedAttribute(attr string, attributes pcommon.Map) (pcommon.Value, bool) {
	left, right, _ := strings.Cut(attr, ".")
	av, ok := attributes.Get(left)
	if !ok {
		return pcommon.Value{}, false
	}

	if len(right) == 0 {
		return av, ok
	}

	return getNestedAttribute(right, av.Map())
}

func parseAttributeNames(attrsToSelect pcommon.Value) []string {
	var out []string

	switch attrsToSelect.Type() {
	case pcommon.ValueTypeStr:
		out = strings.Split(attrsToSelect.AsString(), ",")
	case pcommon.ValueTypeSlice:
		as := attrsToSelect.Slice().AsRaw()
		for _, a := range as {
			out = append(out, fmt.Sprintf("%v", a))
		}
	default:
		// trying to make the most of bad data
		out = append(out, attrsToSelect.AsString())
	}

	return out
}

func removeAttributes(attrs pcommon.Map, labels model.LabelSet) {
	attrs.RemoveIf(func(s string, v pcommon.Value) bool {
		if s == hintAttributes || s == hintResources || s == hintTenant || s == hintFormat {
			return true
		}

		_, exists := labels[model.LabelName(s)]
		return exists
	})
}

func convertLogToJSONEntry(lr plog.LogRecord, res pcommon.Resource, scope pcommon.InstrumentationScope) (*push.Entry, error) {
	line, err := Encode(lr, res, scope)
	if err != nil {
		return nil, err
	}
	return &push.Entry{
		Timestamp: timestampFromLogRecord(lr),
		Line:      line,
	}, nil
}

func convertLogToLogfmtEntry(lr plog.LogRecord, res pcommon.Resource, scope pcommon.InstrumentationScope) (*push.Entry, error) {
	line, err := EncodeLogfmt(lr, res, scope)
	if err != nil {
		return nil, err
	}
	return &push.Entry{
		Timestamp: timestampFromLogRecord(lr),
		Line:      line,
	}, nil
}

func convertLogToLogRawEntry(lr plog.LogRecord) (*push.Entry, error) {
	return &push.Entry{
		Timestamp: timestampFromLogRecord(lr),
		Line:      lr.Body().AsString(),
	}, nil
}

func convertLogToLokiEntry(lr plog.LogRecord, res pcommon.Resource, format string, scope pcommon.InstrumentationScope) (*push.Entry, error) {
	switch format {
	case formatJSON:
		return convertLogToJSONEntry(lr, res, scope)
	case formatLogfmt:
		return convertLogToLogfmtEntry(lr, res, scope)
	case formatRaw:
		return convertLogToLogRawEntry(lr)
	default:
		return nil, fmt.Errorf("invalid format %s. Expected one of: %s, %s, %s", format, formatJSON, formatLogfmt, formatRaw)
	}

}

func timestampFromLogRecord(lr plog.LogRecord) time.Time {
	if lr.Timestamp() != 0 {
		return time.Unix(0, int64(lr.Timestamp()))
	}

	if lr.ObservedTimestamp() != 0 {
		return time.Unix(0, int64(lr.ObservedTimestamp()))
	}

	return time.Unix(0, int64(pcommon.NewTimestampFromTime(timeNow())))
}
